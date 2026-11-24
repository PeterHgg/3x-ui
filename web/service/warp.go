package service

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	warpRegistrationURL = "https://api.cloudflareclient.com/v0a2158/reg"
	warpResponseLimit   = 1 << 20
)

var warpHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// WarpService manages the WARP account stored by the panel, not the host tunnel.
type WarpService struct {
	SettingService
}

type warpAccount struct {
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
	LicenseKey  string `json:"license_key"`
	PrivateKey  string `json:"private_key"`
}

type warpRegistration struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Account struct {
		License string `json:"license"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

func (s *WarpService) GetWarpData() (string, error) {
	return s.SettingService.GetWarp()
}

func (s *WarpService) DelWarpData() error {
	return s.SettingService.SetWarp("")
}

func (s *WarpService) loadWarpAccount() (warpAccount, error) {
	var account warpAccount
	stored, err := s.SettingService.GetWarp()
	if err != nil {
		return account, err
	}
	if stored == "" {
		return account, errors.New("WARP account is not registered")
	}
	if json.Unmarshal([]byte(stored), &account) != nil ||
		strings.TrimSpace(account.DeviceID) == "" || strings.TrimSpace(account.AccessToken) == "" ||
		!validWarpKey(account.PrivateKey) {
		return account, errors.New("invalid stored WARP account data")
	}
	return account, nil
}

// Do not expose upstream bodies, credentials, or device URLs in panel errors.
func requestWarp(ctx context.Context, client *http.Client, method, endpoint, token string, data []byte) (json.RawMessage, error) {
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.New("could not create WARP API request")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method == http.MethodPost {
		req.Header.Set("CF-Client-Version", "a-7.21-0721")
	}
	resp, err := client.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) && requestErr.Timeout() {
			return nil, errors.New("WARP API request timed out")
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not reach the WARP API")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("WARP API returned HTTP %d", resp.StatusCode)
	}
	result, err := io.ReadAll(io.LimitReader(resp.Body, warpResponseLimit+1))
	if err != nil {
		return nil, errors.New("could not read WARP API response (interrupted or timed out)")
	}
	if len(result) > warpResponseLimit {
		return nil, errors.New("WARP API response is too large")
	}
	if !json.Valid(result) {
		return nil, errors.New("WARP API returned invalid JSON")
	}
	return result, nil
}

func validWarpKey(value string) bool {
	key, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(key) == 32
}

func decodeWarpConfig(data json.RawMessage) (warpRegistration, error) {
	var reg warpRegistration
	if err := json.Unmarshal(data, &reg); err != nil {
		return reg, errors.New("invalid WARP device configuration")
	}
	reserved, err := base64.StdEncoding.DecodeString(reg.Config.ClientID)
	if err != nil || len(reserved) != 3 {
		return reg, errors.New("invalid WARP client_id: expected three bytes")
	}
	if len(reg.Config.Peers) == 0 || !validWarpKey(reg.Config.Peers[0].PublicKey) {
		return reg, errors.New("WARP configuration has no valid peer key")
	}
	addresses := reg.Config.Interface.Addresses
	v4, _ := netip.ParseAddr(addresses.V4)
	v6, _ := netip.ParseAddr(addresses.V6)
	if (addresses.V4 != "" && (!v4.Is4() || v4.IsUnspecified())) ||
		(addresses.V6 != "" && (!v6.Is6() || v6.Is4In6() || v6.IsUnspecified() || v6.Zone() != "")) ||
		(addresses.V4 == "" && addresses.V6 == "") {
		return reg, errors.New("WARP configuration has no valid interface addresses")
	}
	return reg, nil
}

func (s *WarpService) GetWarpConfig(ctx context.Context) (string, error) {
	account, err := s.loadWarpAccount()
	if err != nil {
		return "", err
	}
	result, err := requestWarp(ctx, warpHTTPClient, http.MethodGet, warpRegistrationURL+"/"+url.PathEscape(account.DeviceID), account.AccessToken, nil)
	if err != nil {
		return "", err
	}
	if _, err := decodeWarpConfig(result); err != nil {
		return "", err
	}
	return string(result), nil
}

func (s *WarpService) RegWarp(ctx context.Context, secretKey, publicKey string) (string, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return "", errors.New("invalid WARP private key")
	}
	key, err := ecdh.X25519().NewPrivateKey(keyBytes)
	if err != nil || base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()) != publicKey {
		return "", errors.New("invalid WARP key pair")
	}
	hostName, _ := os.Hostname()
	payload, err := json.Marshal(map[string]any{
		"key": publicKey, "tos": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"type": "PC", "model": "x-ui", "name": hostName, "warp_enabled": true,
	})
	if err != nil {
		return "", err
	}
	result, err := requestWarp(ctx, warpHTTPClient, http.MethodPost, warpRegistrationURL, "", payload)
	if err != nil {
		return "", err
	}
	reg, err := decodeWarpConfig(result)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reg.ID) == "" || strings.TrimSpace(reg.Token) == "" {
		return "", errors.New("WARP registration response is missing account data")
	}
	account := warpAccount{reg.Token, reg.ID, reg.Account.License, secretKey}
	// Preserve the existing JSON response structure used by the WARP modal.
	stored, err := json.Marshal(account)
	if err != nil {
		return "", err
	}
	response, err := json.Marshal(struct {
		Data   json.RawMessage `json:"data"`
		Config json.RawMessage `json:"config"`
	}{stored, result})
	if err != nil {
		return "", err
	}
	if err := s.SettingService.SetWarp(string(stored)); err != nil {
		return "", errors.New("could not save the WARP account")
	}
	return string(response), nil
}

func (s *WarpService) SetWarpLicense(ctx context.Context, license string) (string, error) {
	account, err := s.loadWarpAccount()
	if err != nil {
		return "", err
	}
	license = strings.TrimSpace(license)
	if license == "" {
		return "", errors.New("WARP license is empty")
	}
	payload, err := json.Marshal(map[string]string{"license": license})
	if err != nil {
		return "", err
	}
	result, err := requestWarp(ctx, warpHTTPClient, http.MethodPut, warpRegistrationURL+"/"+url.PathEscape(account.DeviceID)+"/account", account.AccessToken, payload)
	if err != nil {
		return "", err
	}
	var response struct {
		Success *bool  `json:"success"`
		License string `json:"license"`
	}
	if json.Unmarshal(result, &response) != nil ||
		(response.Success != nil && !*response.Success) ||
		(response.Success == nil && response.License != license) {
		return "", errors.New("WARP API did not confirm the license update")
	}
	account.LicenseKey = license
	stored, err := json.Marshal(account)
	if err != nil {
		return "", err
	}
	if err := s.SettingService.SetWarp(string(stored)); err != nil {
		return "", errors.New("could not save the updated WARP account")
	}
	return string(stored), nil
}
