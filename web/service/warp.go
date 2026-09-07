package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
)

// WarpService provides business logic for Cloudflare WARP integration.
// It manages WARP configuration and connectivity settings.
type WarpService struct {
	SettingService
}

func (s *WarpService) GetWarpData() (string, error) {
	warp, err := s.SettingService.GetWarp()
	if err != nil {
		return "", err
	}
	return warp, nil
}

func (s *WarpService) DelWarpData() error {
	GetWarpDaemon().Stop()
	err := s.SettingService.SetWarp("")
	if err != nil {
		return err
	}
	return nil
}

func (s *WarpService) GetWarpConfig() (string, error) {
	var warpData map[string]string
	warp, err := s.SettingService.GetWarp()
	if err != nil {
		return "", err
	}
	if warp == "" {
		return "", fmt.Errorf("no WARP data found")
	}
	err = json.Unmarshal([]byte(warp), &warpData)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.cloudflareclient.com/v0a2158/reg/%s", warpData["device_id"])

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buffer := &bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func (s *WarpService) RegWarp(secretKey string, publicKey string) (string, error) {
	tos := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	hostName, _ := os.Hostname()
	data := fmt.Sprintf(`{"key":"%s","tos":"%s","type": "PC","model": "x-ui", "name": "%s","warp_enabled":true}`, publicKey, tos, hostName)

	url := "https://api.cloudflareclient.com/v0a2158/reg"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", err
	}

	req.Header.Add("CF-Client-Version", "a-7.21-0721")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buffer := &bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}

	var rspData map[string]any
	err = json.Unmarshal(buffer.Bytes(), &rspData)
	if err != nil {
		return "", err
	}

	deviceId, _ := rspData["id"].(string)
	token, _ := rspData["token"].(string)
	license, ok := rspData["account"].(map[string]any)["license"].(string)
	if !ok {
		logger.Debug("Error accessing license value.")
		return "", err
	}

	warpData := fmt.Sprintf("{\n  \"access_token\": \"%s\",\n  \"device_id\": \"%s\",", token, deviceId)
	warpData += fmt.Sprintf("\n  \"license_key\": \"%s\",\n  \"private_key\": \"%s\"\n}", license, secretKey)

	s.SettingService.SetWarp(warpData)

	_ = s.StartDaemonFromAccountAndConfig(warpData, buffer.String())

	result := fmt.Sprintf("{\n  \"data\": %s,\n  \"config\": %s\n}", warpData, buffer.String())

	return result, nil
}

func (s *WarpService) SetWarpLicense(license string) (string, error) {
	var warpData map[string]string
	warp, err := s.SettingService.GetWarp()
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(warp), &warpData)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.cloudflareclient.com/v0a2158/reg/%s/account", warpData["device_id"])
	data := fmt.Sprintf(`{"license": "%s"}`, license)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buffer := &bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}

	var response map[string]any
	err = json.Unmarshal(buffer.Bytes(), &response)
	if err != nil {
		return "", err
	}
	if response["success"] == false {
		errorArr, _ := response["errors"].([]any)
		errorObj := errorArr[0].(map[string]any)
		return "", common.NewError(errorObj["code"], errorObj["message"])
	}

	warpData["license_key"] = license
	newWarpData, err := json.MarshalIndent(warpData, "", "  ")
	if err != nil {
		return "", err
	}
	s.SettingService.SetWarp(string(newWarpData))

	return string(newWarpData), nil
}

// StartDaemonFromAccountAndConfig parses WARP registration data and starts the in-process proxy daemon.
func (s *WarpService) StartDaemonFromAccountAndConfig(warpDataStr string, configStr string) error {
	var warpData map[string]string
	if err := json.Unmarshal([]byte(warpDataStr), &warpData); err != nil {
		return err
	}
	var cfgData map[string]any
	if err := json.Unmarshal([]byte(configStr), &cfgData); err != nil {
		return err
	}

	configObj, ok := cfgData["config"].(map[string]any)
	if !ok {
		configObj = cfgData
	}

	clientId, _ := configObj["client_id"].(string)
	var reserved []byte
	if clientId != "" {
		if dec, err := base64.StdEncoding.DecodeString(clientId); err == nil && len(dec) >= 3 {
			reserved = dec[:3]
		}
	}

	peers, _ := configObj["peers"].([]any)
	if len(peers) == 0 {
		return fmt.Errorf("no peers found in WARP config")
	}
	peerObj, _ := peers[0].(map[string]any)
	pubKey, _ := peerObj["public_key"].(string)

	endpointObj, _ := peerObj["endpoint"].(map[string]any)
	endpointStr := "162.159.192.5:2408"
	if v4, ok := endpointObj["v4"].(string); ok && v4 != "" {
		endpointStr = strings.Split(v4, ":")[0] + ":2408"
	} else if host, ok := endpointObj["host"].(string); ok && host != "" {
		endpointStr = host
	}

	ifaceObj, _ := configObj["interface"].(map[string]any)
	addressesObj, _ := ifaceObj["addresses"].(map[string]any)
	var addresses []string
	if v4, ok := addressesObj["v4"].(string); ok && v4 != "" {
		addresses = append(addresses, v4+"/32")
	}
	if v6, ok := addressesObj["v6"].(string); ok && v6 != "" {
		addresses = append(addresses, v6+"/128")
	}

	daemonCfg := &WarpDaemonConfig{
		PrivateKey: warpData["private_key"],
		PublicKey:  pubKey,
		Endpoint:   endpointStr,
		Addresses:  addresses,
		Reserved:   reserved,
		MTU:        1420,
		KeepAlive:  30,
	}

	return GetWarpDaemon().Start(daemonCfg)
}

// StartDaemonFromStoredConfig fetches configuration from Cloudflare and starts the in-process daemon.
func (s *WarpService) StartDaemonFromStoredConfig() error {
	warpData, err := s.GetWarpData()
	if err != nil || warpData == "" {
		return nil
	}
	configStr, err := s.GetWarpConfig()
	if err != nil {
		return err
	}
	return s.StartDaemonFromAccountAndConfig(warpData, configStr)
}
