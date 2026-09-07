package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

const (
	DefaultWarpProxyPort = 40000
)

// WarpDaemon manages an in-process WireGuard-to-SOCKS5 proxy service.
type WarpDaemon struct {
	mu       sync.Mutex
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	listener net.Listener
	device   *device.Device
	tnet     *netstack.Net
	port     int
}

var globalWarpDaemon = &WarpDaemon{
	port: DefaultWarpProxyPort,
}

// GetWarpDaemon returns the global in-process WARP proxy daemon instance.
func GetWarpDaemon() *WarpDaemon {
	return globalWarpDaemon
}

type reservedBind struct {
	conn.Bind
	reserved [3]byte
	hasRes   bool
}

func newReservedBind(bind conn.Bind, reserved []byte) *reservedBind {
	rb := &reservedBind{Bind: bind}
	if len(reserved) == 3 {
		copy(rb.reserved[:], reserved)
		rb.hasRes = true
	}
	return rb
}

func (rb *reservedBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	fns, actualPort, err := rb.Bind.Open(port)
	if err != nil {
		return nil, 0, err
	}
	if !rb.hasRes {
		return fns, actualPort, nil
	}
	wrappedFns := make([]conn.ReceiveFunc, len(fns))
	for i, fn := range fns {
		wrappedFns[i] = func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
			n, err = fn(bufs, sizes, eps)
			if err != nil {
				return n, err
			}
			for j := 0; j < n; j++ {
				if sizes[j] > 3 {
					bufs[j][1] = 0
					bufs[j][2] = 0
					bufs[j][3] = 0
				}
			}
			return n, nil
		}
	}
	return wrappedFns, actualPort, nil
}

func (rb *reservedBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	if rb.hasRes {
		for _, buf := range bufs {
			if len(buf) > 3 {
				copy(buf[1:], rb.reserved[:])
			}
		}
	}
	return rb.Bind.Send(bufs, ep)
}

// WarpDaemonConfig holds parameters needed to establish the in-process WARP tunnel.
type WarpDaemonConfig struct {
	PrivateKey string   `json:"private_key"`
	PublicKey  string   `json:"public_key"`
	Endpoint   string   `json:"endpoint"`
	Addresses  []string `json:"addresses"`
	Reserved   []byte   `json:"reserved"`
	MTU        int      `json:"mtu"`
	KeepAlive  int      `json:"keepalive"`
}

// IsRunning reports whether the WARP daemon is currently running.
func (d *WarpDaemon) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

// Stop stops the in-process WARP daemon and closes all associated network interfaces.
func (d *WarpDaemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}
	d.running = false

	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.listener != nil {
		_ = d.listener.Close()
		d.listener = nil
	}
	if d.device != nil {
		d.device.Close()
		d.device = nil
	}
	d.tnet = nil
	logger.Info("[WARP] In-process WARP daemon stopped")
}

// Start starts the in-process WARP daemon using the provided configuration.
func (d *WarpDaemon) Start(cfg *WarpDaemonConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		d.mu.Unlock()
		d.Stop()
		d.mu.Lock()
	}

	if cfg == nil || cfg.PrivateKey == "" || cfg.PublicKey == "" || cfg.Endpoint == "" {
		return fmt.Errorf("invalid WARP configuration")
	}

	privKeyRaw, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil || len(privKeyRaw) != 32 {
		return fmt.Errorf("invalid private key: %w", err)
	}
	peerKeyRaw, err := base64.StdEncoding.DecodeString(cfg.PublicKey)
	if err != nil || len(peerKeyRaw) != 32 {
		return fmt.Errorf("invalid peer public key: %w", err)
	}

	hexPrivKey := hex.EncodeToString(privKeyRaw)
	hexPeerKey := hex.EncodeToString(peerKeyRaw)

	var localAddrs []netip.Addr
	for _, addrStr := range cfg.Addresses {
		addrClean := strings.Split(addrStr, "/")[0]
		if ip, err := netip.ParseAddr(addrClean); err == nil {
			localAddrs = append(localAddrs, ip)
		}
	}
	if len(localAddrs) == 0 {
		localAddrs = []netip.Addr{
			netip.MustParseAddr("172.16.0.2"),
		}
	}

	dnsAddrs := []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("8.8.8.8"),
	}

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}

	tunDev, tnet, err := netstack.CreateNetTUN(localAddrs, dnsAddrs, mtu)
	if err != nil {
		return fmt.Errorf("failed to create netstack TUN: %w", err)
	}

	bind := newReservedBind(conn.NewDefaultBind(), cfg.Reserved)
	devLogger := device.NewLogger(device.LogLevelError, "[WARP] ")
	dev := device.NewDevice(tunDev, bind, devLogger)

	keepAlive := cfg.KeepAlive
	if keepAlive <= 0 {
		keepAlive = 30
	}

	ipcConfig := fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/0\nallowed_ip=::/0\npersistent_keepalive_interval=%d\n",
		hexPrivKey, hexPeerKey, cfg.Endpoint, keepAlive)

	if err := dev.IpcSet(ipcConfig); err != nil {
		dev.Close()
		return fmt.Errorf("failed to configure WireGuard device: %w", err)
	}

	if err := dev.Up(); err != nil {
		dev.Close()
		return fmt.Errorf("failed to bring up WireGuard device: %w", err)
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", d.port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		dev.Close()
		return fmt.Errorf("failed to bind SOCKS5 listener on %s: %w", listenAddr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.ctx = ctx
	d.cancel = cancel
	d.listener = listener
	d.device = dev
	d.tnet = tnet
	d.running = true

	logger.Infof("[WARP] In-process WARP SOCKS5 daemon listening on %s (MTU: %d, Endpoint: %s)", listenAddr, mtu, cfg.Endpoint)

	go d.acceptLoop(listener, ctx)

	return nil
}

func (d *WarpDaemon) acceptLoop(l net.Listener, ctx context.Context) {
	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				return
			}
		}
		go d.handleSocks5(conn)
	}
}

func (d *WarpDaemon) handleSocks5(client net.Conn) {
	defer client.Close()

	_ = client.SetDeadline(time.Now().Add(10 * time.Second))

	// 1. Negotiation
	buf := make([]byte, 258)
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return
	}
	nmethods := int(buf[1])
	if nmethods <= 0 || nmethods > 255 {
		return
	}
	if _, err := io.ReadFull(client, buf[:nmethods]); err != nil {
		return
	}
	if _, err := client.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// 2. Request
	if _, err := io.ReadFull(client, buf[:4]); err != nil {
		return
	}
	if buf[0] != 0x05 || buf[1] != 0x01 { // Only CONNECT cmd supported
		_, _ = client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var targetHost string
	switch buf[3] { // ATYP
	case 0x01: // IPv4
		if _, err := io.ReadFull(client, buf[:4]); err != nil {
			return
		}
		targetHost = net.IP(buf[:4]).String()
	case 0x03: // Domain name
		if _, err := io.ReadFull(client, buf[:1]); err != nil {
			return
		}
		domainLen := int(buf[0])
		if domainLen <= 0 {
			return
		}
		if _, err := io.ReadFull(client, buf[:domainLen]); err != nil {
			return
		}
		targetHost = string(buf[:domainLen])
	case 0x04: // IPv6
		if _, err := io.ReadFull(client, buf[:16]); err != nil {
			return
		}
		targetHost = net.IP(buf[:16]).String()
	default:
		_, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return
	}
	targetPort := int(buf[0])<<8 | int(buf[1])
	targetAddr := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))

	d.mu.Lock()
	tnet := d.tnet
	d.mu.Unlock()

	if tnet == nil {
		_, _ = client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer dialCancel()

	targetConn, err := tnet.DialContext(dialCtx, "tcp", targetAddr)
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	if _, err := client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	_ = client.SetDeadline(time.Time{})

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(targetConn, client)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(client, targetConn)
		errc <- err
	}()
	<-errc
}
