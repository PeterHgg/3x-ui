package job

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type AliyunDNSJob struct {
	settingService service.SettingService
}

type WetestResponse struct {
	Status int `json:"status"`
	Data   struct {
		CM []string `json:"cm"` // Mobile
		CU []string `json:"cu"` // Unicom
		CT []string `json:"ct"` // Telecom
	} `json:"data"`
}

func NewAliyunDNSJob() *AliyunDNSJob {
	return &AliyunDNSJob{}
}

func (j *AliyunDNSJob) Run() {
	enabled, _ := j.settingService.GetClashXcdnEnabled()
	if !enabled {
		return
	}

	ak, _ := j.settingService.GetAliyunAk()
	sk, _ := j.settingService.GetAliyunSk()
	if ak == "" || sk == "" {
		logger.Debug("AliyunDNSJob: Aliyun AK/SK not configured, skipping")
		return
	}

	mainDomain, _ := j.settingService.GetClashDomain()
	if mainDomain == "" {
		// Fallback to sub domain
		mainDomain, _ = j.settingService.GetSubDomain()
	}
	if mainDomain == "" {
		logger.Warning("AliyunDNSJob: Clash domain not configured, skipping")
		return
	}

	prefix, _ := j.settingService.GetClashPrefix()
	if prefix == "" {
		prefix = "cdn"
	}
	// xcdn node record name
	recordName := "x" + prefix

	logger.Infof("AliyunDNSJob: Starting DNS sync for %s.%s", recordName, mainDomain)

	// 1. Fetch IPs from Wetest
	ips, err := j.fetchBestIPs()
	if err != nil {
		logger.Errorf("AliyunDNSJob: Failed to fetch IPs from wetest: %v", err)
		return
	}

	// 2. Update Aliyun DNS
	err = j.syncAliyunDNS(ak, sk, mainDomain, recordName, ips)
	if err != nil {
		logger.Errorf("AliyunDNSJob: Failed to sync Aliyun DNS: %v", err)
	} else {
		logger.Infof("AliyunDNSJob: Successfully synced DNS records for %s.%s", recordName, mainDomain)
	}
}

func (j *AliyunDNSJob) fetchBestIPs() (*WetestResponse, error) {
	apiUrl := "https://www.wetest.vip/api/cf2dns/get_cloudflare_ip?key=o1zrmHAF&type=v4"
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wetestResp WetestResponse
	err = json.Unmarshal(body, &wetestResp)
	if err != nil {
		return nil, err
	}

	if wetestResp.Status != 200 && wetestResp.Status != 1 {
		return nil, fmt.Errorf("wetest api error status: %d", wetestResp.Status)
	}

	return &wetestResp, nil
}

func (j *AliyunDNSJob) syncAliyunDNS(ak, sk, domain, record string, ips *WetestResponse) error {
	client := NewAliyunDNSClient(ak, sk)

	syncLine := func(line string, newIPs []string) error {
		if len(newIPs) == 0 {
			return nil
		}

		// Get existing records for this line
		existingRecords, err := client.GetRecords(domain, record, line)
		if err != nil {
			return err
		}

		existingIPMap := make(map[string]string) // ip -> recordId
		for _, r := range existingRecords {
			existingIPMap[r.Value] = r.RecordId
		}

		newIPMap := make(map[string]bool)
		for _, ip := range newIPs {
			newIPMap[ip] = true
			if _, exists := existingIPMap[ip]; !exists {
				// Add new record
				err := client.AddRecord(domain, record, "A", ip, line)
				if err != nil {
					logger.Warningf("AliyunDNSJob: Failed to add record %s for %s: %v", ip, line, err)
				}
			}
		}

		// Remove records not in the new list
		for ip, recordId := range existingIPMap {
			if !newIPMap[ip] {
				err := client.DeleteRecord(recordId)
				if err != nil {
					logger.Warningf("AliyunDNSJob: Failed to delete record %s (%s): %v", ip, recordId, err)
				}
			}
		}

		return nil
	}

	// Aliyun line names (standard)
	err := syncLine("telecom", ips.Data.CT)
	if err != nil {
		return err
	}
	err = syncLine("unicom", ips.Data.CU)
	if err != nil {
		return err
	}
	err = syncLine("mobile", ips.Data.CM)
	if err != nil {
		return err
	}

	return nil
}

type AliyunDNSClient struct {
	ak string
	sk string
}

func NewAliyunDNSClient(ak, sk string) *AliyunDNSClient {
	return &AliyunDNSClient{ak: ak, sk: sk}
}

type AliyunRecord struct {
	RecordId string `json:"RecordId"`
	Value    string `json:"Value"`
	Line     string `json:"Line"`
}

type AliyunResponse struct {
	DomainRecords struct {
		Record []AliyunRecord `json:"Record"`
	} `json:"DomainRecords"`
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

func (c *AliyunDNSClient) doRequest(params url.Values) ([]byte, error) {
	params.Set("Format", "JSON")
	params.Set("Version", "2015-01-09")
	params.Set("AccessKeyId", c.ak)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", uuid.New().String())

	// Sorting keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// CanonicalizedQueryString
	var canonicalizedQueryString strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonicalizedQueryString.WriteString("&")
		}
		canonicalizedQueryString.WriteString(url.QueryEscape(k))
		canonicalizedQueryString.WriteString("=")
		canonicalizedQueryString.WriteString(url.QueryEscape(params.Get(k)))
	}

	// StringToSign
	stringToSign := "GET&%2F&" + url.QueryEscape(canonicalizedQueryString.String())
	stringToSign = strings.ReplaceAll(stringToSign, "+", "%20")
	stringToSign = strings.ReplaceAll(stringToSign, "*", "%2A")
	stringToSign = strings.ReplaceAll(stringToSign, "%7E", "~")

	// HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(c.sk+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	params.Set("Signature", signature)

	url := "https://alidns.aliyuncs.com/?" + params.Encode()
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *AliyunDNSClient) GetRecords(domain, record, line string) ([]AliyunRecord, error) {
	params := url.Values{}
	params.Set("Action", "DescribeDomainRecords")
	params.Set("DomainName", domain)
	params.Set("RRKeyWord", record)
	params.Set("Line", line)

	data, err := c.doRequest(params)
	if err != nil {
		return nil, err
	}

	var resp AliyunResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != "" {
		return nil, fmt.Errorf("aliyun error: %s - %s", resp.Code, resp.Message)
	}

	var results []AliyunRecord
	for _, r := range resp.DomainRecords.Record {
		if r.Line == line && r.Value != "" {
			results = append(results, r)
		}
	}
	return results, nil
}

func (c *AliyunDNSClient) AddRecord(domain, record, typeStr, value, line string) error {
	params := url.Values{}
	params.Set("Action", "AddDomainRecord")
	params.Set("DomainName", domain)
	params.Set("RR", record)
	params.Set("Type", typeStr)
	params.Set("Value", value)
	params.Set("Line", line)

	data, err := c.doRequest(params)
	if err != nil {
		return err
	}

	var resp AliyunResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return err
	}

	if resp.Code != "" {
		return fmt.Errorf("aliyun error: %s - %s", resp.Code, resp.Message)
	}

	return nil
}

func (c *AliyunDNSClient) DeleteRecord(recordId string) error {
	params := url.Values{}
	params.Set("Action", "DeleteDomainRecord")
	params.Set("RecordId", recordId)

	data, err := c.doRequest(params)
	if err != nil {
		return err
	}

	var resp AliyunResponse
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return err
	}

	if resp.Code != "" {
		return fmt.Errorf("aliyun error: %s - %s", resp.Code, resp.Message)
	}

	return nil
}

