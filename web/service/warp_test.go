package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeWarpConfig(t *testing.T) {
	peerKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	for _, tc := range []struct {
		name, clientID, peers, addresses string
		wantErr                          bool
	}{
		{"IPv4 and zero reserved bytes", "AAEA", fmt.Sprintf(`[{"public_key":%q}]`, peerKey), `{"v4":"172.16.0.2"}`, false},
		{"IPv6 only", "AAAA", fmt.Sprintf(`[{"public_key":%q}]`, peerKey), `{"v6":"2001:db8::1"}`, false},
		{"missing peer", "AAAA", `[]`, `{"v4":"172.16.0.2"}`, true},
		{"invalid key", "AAAA", `[{"public_key":"invalid"}]`, `{"v4":"172.16.0.2"}`, true},
		{"short client ID", "AA==", fmt.Sprintf(`[{"public_key":%q}]`, peerKey), `{"v4":"172.16.0.2"}`, true},
		{"wrong address family", "AAAA", fmt.Sprintf(`[{"public_key":%q}]`, peerKey), `{"v6":"172.16.0.2"}`, true},
		{"missing addresses", "AAAA", fmt.Sprintf(`[{"public_key":%q}]`, peerKey), `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := json.RawMessage(fmt.Sprintf(`{"config":{"client_id":%q,"peers":%s,"interface":{"addresses":%s}}}`, tc.clientID, tc.peers, tc.addresses))
			config, err := decodeWarpConfig(data)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if err == nil && config.Config.ClientID != tc.clientID {
				t.Fatal("client ID changed during validation")
			}
		})
	}
}

func TestRequestWarpRejectsBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		body, wantErr string
	}{
		{"valid response", 200, `{"ok":true}`, ""},
		{"upstream rejection", 429, `{"token":"sensitive-test-data"}`, "WARP API returned HTTP 429"},
		{"invalid JSON", 200, "<html>sensitive-test-data</html>", "WARP API returned invalid JSON"},
		{"oversized response", 200, strings.Repeat("x", warpResponseLimit+1), "WARP API response is too large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			result, err := requestWarp(context.Background(), server.Client(), http.MethodGet, server.URL, "test-token", nil)
			if tc.wantErr == "" {
				if err != nil || string(result) != tc.body {
					t.Fatalf("request failed: %v", err)
				}
			} else if err == nil || err.Error() != tc.wantErr || result != nil {
				t.Fatalf("expected sanitized error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRequestWarpTimeoutAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 75 * time.Millisecond
	if _, err := requestWarp(context.Background(), client, http.MethodGet, server.URL, "", nil); err == nil || err.Error() != "WARP API request timed out" {
		t.Fatalf("expected timeout error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := requestWarp(ctx, client, http.MethodGet, server.URL, "", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestRegWarpRejectsMismatchedKeysBeforeNetwork(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	if _, err := (&WarpService{}).RegWarp(context.Background(), key, key); err == nil || err.Error() != "invalid WARP key pair" {
		t.Fatalf("expected key pair rejection before request, got %v", err)
	}
}
