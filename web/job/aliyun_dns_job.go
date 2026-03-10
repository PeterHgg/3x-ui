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
	_, _ = j.Sync()
}

func (j *AliyunDNSJob) Sync() ([]string, error) {
	var logs []string
	addLog := func(msg string) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
		logger.Info("AliyunDNSJob:", msg)
	}

	enabled, _ := j.settingService.GetClashXcdnEnabled()
	if !enabled {
		return logs, fmt.Errorf("Clash XCDN feature not enabled")
	}

	ak, _ := j.settingService.GetAliyunAk()
	sk, _ := j.settingService.GetAliyunSk()
	if ak == "" || sk == "" {
		return logs, fmt.Errorf("Aliyun AK/SK not configured")
	}

	mainDomain, _ := j.settingService.GetClashDomain()
	if mainDomain == "" {
		mainDomain, _ = j.settingService.GetSubDomain()
	}
	if mainDomain == "" {
		return logs, fmt.Errorf("Clash domain not configured")
	}

	prefix, _ := j.settingService.GetClashPrefix()
	if prefix == "" {
		prefix = "cdn"
	}
	recordName := "x" + prefix

	addLog(fmt.Sprintf("Starting DNS sync for %s.%s", recordName, mainDomain))

	// 1. Fetch IPs from Wetest
	addLog("Fetching best IPs from wetest.vip...")
	ips, err := j.fetchBestIPs()
	if err != nil {
		errMSg := fmt.Sprintf("Failed to fetch IPs: %v", err)
		addLog(errMSg)
		return logs, err
	}
	addLog(fmt.Sprintf("Fetched IPs - Telecom: %d, Unicom: %d, Mobile: %d",
		len(ips.Data.CT), len(ips.Data.CU), len(ips.Data.CM)))

	// 2. Update Aliyun DNS
	addLog("Updating Aliyun DNS records...")
	syncLine := func(line string, newIPs []string) error {
		if len(newIPs) == 0 {
			return nil
		}
		addLog(fmt.Sprintf("Syncing line [%s] with %d IPs...", line, len(newIPs)))
		client := NewAliyunDNSClient(ak, sk)
		existingRecords, err := client.GetRecords(mainDomain, recordName, line)
		if err != nil {
			return err
		}

		existingIPMap := make(map[string]string)
		for _, r := range existingRecords {
			existingIPMap[r.Value] = r.RecordId
		}

		newIPMap := make(map[string]bool)
		for _, ip := range newIPs {
			newIPMap[ip] = true
			if _, exists := existingIPMap[ip]; !exists {
				addLog(fmt.Sprintf("Adding record: %s -> %s", line, ip))
				err := client.AddRecord(mainDomain, recordName, "A", ip, line)
				if err != nil {
					addLog(fmt.Sprintf("Error adding %s: %v", ip, err))
				}
			}
		}

		for ip, recordId := range existingIPMap {
			if !newIPMap[ip] {
				addLog(fmt.Sprintf("Deleting old record: %s -> %s", line, ip))
				err := client.DeleteRecord(recordId)
				if err != nil {
					addLog(fmt.Sprintf("Error deleting %s: %v", ip, err))
				}
			}
		}
		return nil
	}

	if err := syncLine("telecom", ips.Data.CT); err != nil {
		addLog(fmt.Sprintf("Telecom sync error: %v", err))
	}
	if err := syncLine("unicom", ips.Data.CU); err != nil {
		addLog(fmt.Sprintf("Unicom sync error: %v", err))
	}
	if err := syncLine("mobile", ips.Data.CM); err != nil {
		addLog(fmt.Sprintf("Mobile sync error: %v", err))
	}

	addLog("DNS sync completed successfully")
	return logs, nil
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

