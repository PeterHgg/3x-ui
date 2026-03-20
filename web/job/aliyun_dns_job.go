package job

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

type WetestIP struct {
	IP string `json:"ip"`
}

type WetestData struct {
	CM []WetestIP `json:"CM"`
	Cm []WetestIP `json:"cm"`
	CU []WetestIP `json:"CU"`
	Cu []WetestIP `json:"cu"`
	CT []WetestIP `json:"CT"`
	Ct []WetestIP `json:"ct"`
}

type WetestResponse struct {
	Status any       `json:"status"`
	Code   int       `json:"code"`
	Msg    string    `json:"msg"`
	Data   WetestData `json:"data"`
	Info   WetestData `json:"info"`
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

	if ak == "" || sk == "" {
		addLog("Aliyun AK/SK 为空，跳过 CNAME 兜底")
		return logs, fmt.Errorf("Aliyun AK/SK 未配置")
	}

	client := NewAliyunDNSClient(ak, sk)
	fallbackLine := "中国地区"
	fallbackTargets := []string{
		"www.visa.com",
		"singapore.com",
		"time.is",
		"www.whoer.net",
		"store.epicgames.com",
		"www.wto.org",
		"www.whatismyip.com",
		"www.ipget.net",
		"icook.hk",
		"japan.com",
	}
	currentRecords, err := client.GetRecords(mainDomain, recordName, "")
	if err != nil {
		addLog(fmt.Sprintf("获取当前 DNS 记录失败: %v", err))
		return logs, err
	}

	hasARecord := false
	for _, r := range currentRecords {
		if strings.EqualFold(r.Type, "A") {
			hasARecord = true
			break
		}
	}
	if !hasARecord {
		addLog("当前无 A 记录，直接写入 CNAME 兜底解析")
		if err := j.syncCnameFallback(client, mainDomain, recordName, fallbackLine, fallbackTargets, addLog); err != nil {
			addLog(fmt.Sprintf("CNAME 兜底失败: %v", err))
			return logs, err
		}
		addLog("CNAME 兜底完成")
		return logs, nil
	}

	failCount, _ := j.settingService.GetAliyunDnsFailCount()
	failedThisRun := false
	applyFailure := func(reason string, err error, statusCode int, rawBody string) error {
		if !failedThisRun {
			failCount++
			failedThisRun = true
			_ = j.settingService.UpdateAliyunDnsFailCount(failCount)
		}
		addLog(fmt.Sprintf("%s，连续失败次数: %d/5", reason, failCount))
		if rawBody != "" {
			addLog(fmt.Sprintf("Wetest response status=%d, body_len=%d", statusCode, len(rawBody)))
		}
		if failCount >= 5 {
			addLog("连续失败达到阈值，切换 CNAME 兜底")
			if err := j.syncCnameFallback(client, mainDomain, recordName, fallbackLine, fallbackTargets, addLog); err != nil {
				addLog(fmt.Sprintf("CNAME 兜底失败: %v", err))
				return err
			}
			addLog("CNAME 兜底完成")
		}
		return err
	}

	// 1. Fetch IPs from Wetest
	addLog("Fetching best IPs from wetest.vip...")
	ips, statusCode, rawBody, err := j.fetchBestIPs()
	if err != nil {
		errMSg := fmt.Sprintf("Failed to fetch IPs: %v", err)
		addLog(errMSg)
		return logs, applyFailure("Wetest 获取 IP 失败", err, statusCode, rawBody)
	}
	toIPs := func(list []WetestIP) []string {
		res := make([]string, 0, len(list))
		for _, item := range list {
			if item.IP != "" {
				res = append(res, item.IP)
			}
		}
		return res
	}
	pickIPs := func(candidates ...[]WetestIP) []string {
		for _, c := range candidates {
			if len(c) > 0 {
				return toIPs(c)
			}
		}
		return nil
	}

	ctIPs := pickIPs(ips.Data.CT, ips.Data.Ct, ips.Info.CT, ips.Info.Ct)
	cuIPs := pickIPs(ips.Data.CU, ips.Data.Cu, ips.Info.CU, ips.Info.Cu)
	cmIPs := pickIPs(ips.Data.CM, ips.Data.Cm, ips.Info.CM, ips.Info.Cm)

	addLog(fmt.Sprintf("Wetest response (status=%d), body_len=%d", statusCode, len(rawBody)))
	addLog(fmt.Sprintf("Fetched IP count - Telecom: %d, Unicom: %d, Mobile: %d",
		len(ctIPs), len(cuIPs), len(cmIPs)))
	if len(ctIPs) == 0 && len(cuIPs) == 0 && len(cmIPs) == 0 {
		err := fmt.Errorf("wetest returned empty IP list")
		addLog("No IPs returned from wetest, aborting DNS sync")
		return logs, applyFailure("Wetest 返回空列表", err, statusCode, rawBody)
	}

	// 2. Update Aliyun DNS
	addLog("Updating Aliyun DNS records...")
	syncLine := func(line string, newIPs []string) error {
		addLog(fmt.Sprintf("Syncing line [%s]...", line))
		if len(newIPs) == 0 {
			addLog(fmt.Sprintf("No IPs returned for line [%s], skipping", line))
			return nil
		}
		addLog(fmt.Sprintf("Line [%s] target IPs: %v", line, newIPs))

		existingRecords, err := client.GetRecords(mainDomain, recordName, line)
		if err != nil {
			return err
		}
		addLog(fmt.Sprintf("Line [%s] existing records: %d", line, len(existingRecords)))

		existingIPMap := make(map[string]string)
		for _, r := range existingRecords {
			addLog(fmt.Sprintf("Line [%s] existing record: %s -> %s (id=%s)", line, recordName, r.Value, r.RecordId))
			existingIPMap[r.Value] = r.RecordId
		}

		var syncErr error
		newIPMap := make(map[string]bool)
		for _, ip := range newIPs {
			newIPMap[ip] = true
			if _, exists := existingIPMap[ip]; !exists {
				addLog(fmt.Sprintf("Adding record: %s -> %s", line, ip))
				err := client.AddRecord(mainDomain, recordName, "A", ip, line)
				if err != nil {
					addLog(fmt.Sprintf("Error adding %s: %v", ip, err))
					if syncErr == nil {
						syncErr = err
					}
				}
			} else {
				addLog(fmt.Sprintf("Keep record: %s -> %s", line, ip))
			}
		}

		for ip, recordId := range existingIPMap {
			if !newIPMap[ip] {
				addLog(fmt.Sprintf("Deleting old record: %s -> %s", line, ip))
				err := client.DeleteRecord(recordId)
				if err != nil {
					addLog(fmt.Sprintf("Error deleting %s: %v", ip, err))
					if syncErr == nil {
						syncErr = err
					}
				}
			}
		}
		return syncErr
	}

	var syncErr error
	if err := syncLine("telecom", ctIPs); err != nil {
		addLog(fmt.Sprintf("Telecom sync error: %v", err))
		syncErr = err
	}
	if err := syncLine("unicom", cuIPs); err != nil {
		addLog(fmt.Sprintf("Unicom sync error: %v", err))
		syncErr = err
	}
	if err := syncLine("mobile", cmIPs); err != nil {
		addLog(fmt.Sprintf("Mobile sync error: %v", err))
		syncErr = err
	}

	if syncErr != nil {
		return logs, applyFailure("Aliyun API 同步失败", syncErr, 0, "")
	}

	_ = j.settingService.UpdateAliyunDnsFailCount(0)
	addLog("DNS sync completed successfully")
	return logs, nil
}

func (j *AliyunDNSJob) syncCnameFallback(client *AliyunDNSClient, domain, recordName, line string, targets []string, addLog func(string)) error {
	if len(targets) == 0 {
		return nil
	}

	existingRecords, err := client.GetRecords(domain, recordName, line)
	if err != nil {
		return err
	}

	keepMap := make(map[string]bool)
	for _, target := range targets {
		keepMap[target] = true
	}

	existingMap := make(map[string]string)
	for _, r := range existingRecords {
		if r.Value == "" {
			continue
		}
		existingMap[r.Value] = r.RecordId
		if !keepMap[r.Value] {
			addLog(fmt.Sprintf("Deleting old record: %s -> %s", line, r.Value))
			if err := client.DeleteRecord(r.RecordId); err != nil {
				addLog(fmt.Sprintf("Error deleting %s: %v", r.Value, err))
			}
		}
	}

	for _, target := range targets {
		if _, exists := existingMap[target]; exists {
			addLog(fmt.Sprintf("Keep record: %s -> %s", line, target))
			continue
		}
		addLog(fmt.Sprintf("Adding record: %s -> %s", line, target))
		if err := client.AddRecord(domain, recordName, "CNAME", target, line); err != nil {
			addLog(fmt.Sprintf("Error adding %s: %v", target, err))
		}
	}

	return nil
}

func (j *AliyunDNSJob) fetchBestIPs() (*WetestResponse, int, string, error) {
	apiUrl := "https://www.wetest.vip/api/cf2dns/get_cloudflare_ip?key=o1zrmHAF&type=v4"
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Get(apiUrl)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, "", err
	}
	rawBody := string(body)

	var wetestResp WetestResponse
	err = json.Unmarshal(body, &wetestResp)
	if err != nil {
		return nil, resp.StatusCode, rawBody, err
	}

	statusOK := false
	switch v := wetestResp.Status.(type) {
	case int:
		if v == 200 || v == 1 {
			statusOK = true
		}
	case float64: // JSON numbers unmarshal to float64 for 'any'
		if v == 200 || v == 1 {
			statusOK = true
		}
	case bool:
		if v {
			statusOK = true
		}
	}

	if !statusOK && wetestResp.Code != 200 && wetestResp.Code != 1 {
		return nil, resp.StatusCode, rawBody, fmt.Errorf("wetest api error status: %v code: %d msg: %s", wetestResp.Status, wetestResp.Code, wetestResp.Msg)
	}

	return &wetestResp, resp.StatusCode, rawBody, nil
}

type AliyunDNSClient struct {
	ak         string
	sk         string
	httpClient *http.Client
}

func NewAliyunDNSClient(ak, sk string) *AliyunDNSClient {
	return &AliyunDNSClient{
		ak: ak,
		sk: sk,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			},
		},
	}
}

type AliyunRecord struct {
	RecordId        string      `json:"RecordId"`
	RR              string      `json:"RR"`
	Type            string      `json:"Type"`
	Value           string      `json:"Value"`
	Line            string      `json:"Line"`
	UpdateTimestamp json.Number `json:"UpdateTimestamp"`
	UpdateTime      string      `json:"UpdateTime"`
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
	resp, err := c.httpClient.Get(url)
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
	if line != "" {
		params.Set("Line", line)
	}

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
		if r.Value == "" {
			continue
		}
		if line != "" && r.Line != line {
			continue
		}
		results = append(results, r)
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

