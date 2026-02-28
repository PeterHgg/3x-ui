package sub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type ClashService struct {
	ruleCache     *RuleCache
	clientService *service.ClientService
}

// 规则缓存
type RuleCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedRule
}

type CachedRule struct {
	Content   string
	UpdatedAt time.Time
}

func NewClashService() *ClashService {
	return &ClashService{
		ruleCache: &RuleCache{
			cache: make(map[string]*CachedRule),
		},
		clientService: &service.ClientService{},
	}
}

// 生成 Clash 配置
func (s *ClashService) GenerateClashConfig(uuid, password, cdnDomain string, count int, prefix, origin string, subPort int, customRules string) (*ClashConfig, error) {
	var baseNodes []*model.Inbound

	if uuid != "" {
		baseNodes = s.findNodesByUUID(uuid)
	} else if password != "" {
		baseNodes = s.findNodesByPassword(password)
	}

	if len(baseNodes) == 0 {
		return nil, fmt.Errorf("未找到对应的节点")
	}

	// 生成 CDN 节点（按备注分组）
	proxiesMap, orderedGroupNames := s.generateCDNProxies(baseNodes, cdnDomain, count, prefix, subPort)

	// 生成代理组
	proxyGroups := s.generateProxyGroups(proxiesMap, orderedGroupNames)

	// 生成规则提供者
	ruleProviders := s.generateRuleProviders(origin)

	// 生成规则（合并自定义规则）
	rules := s.generateRules(customRules)

	// 展平所有代理用于配置文件
	var allProxies []ClashProxy
	for _, ps := range proxiesMap {
		allProxies = append(allProxies, ps...)
	}

	return &ClashConfig{
		MixedPort:          7890,
		AllowLan:           true,
		Mode:               "rule",
		LogLevel:           "info",
		ExternalController: ":9090",
		UnifiedDelay:       true,
		TCPConcurrent:      true,
		DNS: ClashDNS{
			Enable:       true,
			EnhancedMode: "fake-ip",
			FakeIPRange:  "198.18.0.1/16",
			Nameserver:   []string{"223.5.5.5", "119.29.29.29"},
		},
		Profile: ClashProfile{
			StoreSelected: true, // 存储节点选择
			Tracing:       false,
			Interval:      12, // 12小时自动更新
		},
		Proxies:       allProxies,
		ProxyGroups:   proxyGroups,
		RuleProviders: ruleProviders,
		Rules:         rules,
	}, nil
}

// 根据 UUID 查找节点
func (s *ClashService) findNodesByUUID(uuid string) []*model.Inbound {
	inbounds, _ := s.clientService.FindInboundsByClientUUID(uuid)
	return inbounds
}

// 根据密码查找节点
func (s *ClashService) findNodesByPassword(password string) []*model.Inbound {
	inbounds, _ := s.clientService.FindInboundsByClientPassword(password)
	return inbounds
}

// 生成 CDN 节点，返回proxiesMap和按inbound ID排序的组名列表
func (s *ClashService) generateCDNProxies(baseNodes []*model.Inbound, cdnDomain string, count int, prefix string, subPort int) (map[string][]ClashProxy, []string) {
	proxiesMap := make(map[string][]ClashProxy)
	groupIDMap := make(map[string]int) // 记录每个组名对应的最小inbound ID

	for _, inbound := range baseNodes {
		groupName := inbound.Remark
		if groupName == "" {
			groupName = "Default"
		}

		// 记录第一次出现的inbound ID（用于排序）
		if _, exists := groupIDMap[groupName]; !exists {
			groupIDMap[groupName] = inbound.Id
		}

		for i := 1; i <= count; i++ {
			cdnServer := fmt.Sprintf("%d%s.%s", i, prefix, cdnDomain)

			var proxy ClashProxy
			if inbound.Protocol == "vmess" {
				proxy = s.createVMessProxy(inbound, cdnServer, i, prefix, subPort)
			} else if inbound.Protocol == "trojan" {
				proxy = s.createTrojanProxy(inbound, cdnServer, i, prefix, subPort)
			}

			if proxy.Name != "" {
				proxiesMap[groupName] = append(proxiesMap[groupName], proxy)
			}
		}
	}

	// 按inbound ID排序组名
	var orderedGroupNames []string
	for name := range groupIDMap {
		orderedGroupNames = append(orderedGroupNames, name)
	}
	sort.Slice(orderedGroupNames, func(i, j int) bool {
		return groupIDMap[orderedGroupNames[i]] < groupIDMap[orderedGroupNames[j]]
	})

	return proxiesMap, orderedGroupNames
}

// 获取 WebSocket 路径
func (s *ClashService) getWebSocketPath(streamSettingsStr string) string {
	var streamSettings map[string]interface{}
	if err := json.Unmarshal([]byte(streamSettingsStr), &streamSettings); err != nil {
		return "/"
	}

	if wsSettings, ok := streamSettings["wsSettings"].(map[string]interface{}); ok {
		if path, ok := wsSettings["path"].(string); ok && path != "" {
			return path
		}
	}
	return "/"
}

// 创建 VMess 代理
func (s *ClashService) createVMessProxy(inbound *model.Inbound, cdnServer string, index int, prefix string, subPort int) ClashProxy {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	clients, _ := settings["clients"].([]interface{})
	if len(clients) == 0 {
		return ClashProxy{}
	}

	client, _ := clients[0].(map[string]interface{})
	uuid, _ := client["id"].(string)

	// 使用节点备注作为后缀
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%d%s%s", index, prefix, suffix)

	return ClashProxy{
		Name:    name,
		Type:    "vmess",
		Server:  cdnServer,
		Port:    443,
		UUID:    uuid,
		AlterID: 0,
		Cipher:  "auto",
		UDP:     true,
		TLS:     true,
		Network: "ws",
		WSOptions: &ClashWSOptions{
			Path: s.getWebSocketPath(inbound.StreamSettings),
		},
	}
}

// 创建 Trojan 代理
func (s *ClashService) createTrojanProxy(inbound *model.Inbound, cdnServer string, index int, prefix string, subPort int) ClashProxy {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	clients, _ := settings["clients"].([]interface{})
	if len(clients) == 0 {
		return ClashProxy{}
	}

	client, _ := clients[0].(map[string]interface{})
	password, _ := client["password"].(string)

	// 使用节点备注作为后缀
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%d%s%s", index, prefix, suffix)

	return ClashProxy{
		Name:           name,
		Type:           "trojan",
		Server:         cdnServer,
		Port:           443,
		Password:       password,
		SkipCertVerify: true,
		UDP:            true,
		Network:        "ws",
		WSOptions: &ClashWSOptions{
			Path: s.getWebSocketPath(inbound.StreamSettings),
		},
	}
}


// 生成代理组（使用按inbound ID排序的组名列表）
func (s *ClashService) generateProxyGroups(proxiesMap map[string][]ClashProxy, orderedGroupNames []string) []ClashProxyGroup {
	groups := []ClashProxyGroup{}

	// 1. 创建 "🚀 手动切换" 组 (放在最前面)
	selectGroup := ClashProxyGroup{
		Name:    "🚀 手动切换",
		Type:    "select",
		Proxies: []string{}, // 稍后设置
	}
	groups = append(groups, selectGroup)

	// 手动切换组的 Proxies 列表
	var topLevelProxies []string

	// 2. 按排序后的顺序创建 load-balance 组和 url-test 单节点组
	for _, groupName := range orderedGroupNames {
		proxies, ok := proxiesMap[groupName]
		if !ok {
			continue
		}

		var proxyNames []string
		for _, p := range proxies {
			proxyNames = append(proxyNames, p.Name)
		}

		groups = append(groups, ClashProxyGroup{
			Name:     groupName,
			Type:     "load-balance",
			Proxies:  proxyNames,
			URL:      "http://cp.cloudflare.com/generate_204",
			Interval: 300,
			Strategy: "round-robin", // 显式设置为 round-robin
		})

		// 创建对应的 url-test 单节点组
		singleNodeGroupName := groupName + " 单节点"
		groups = append(groups, ClashProxyGroup{
			Name:     singleNodeGroupName,
			Type:     "url-test",
			Proxies:  proxyNames,
			URL:      "http://cp.cloudflare.com/generate_204",
			Interval: 300,
		})

		// load-balance 组和单节点组都加入手动切换
		topLevelProxies = append(topLevelProxies, groupName)
		topLevelProxies = append(topLevelProxies, singleNodeGroupName)
	}

	// 更新 "手动切换" 组的 proxies
	groups[0].Proxies = topLevelProxies

	// 3. 创建 "🎯 兜底规则" 组，用于 MATCH 兜底
	fallbackGroup := ClashProxyGroup{
		Name:    "🎯 兜底规则",
		Type:    "select",
		Proxies: []string{"🚀 手动切换", "DIRECT"},
	}
	groups = append(groups, fallbackGroup)

	return groups
}

// 生成规则提供者
func (s *ClashService) generateRuleProviders(origin string) map[string]ClashRuleProvider {
	return map[string]ClashRuleProvider{
		"proxy": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/proxy", origin),
			Path:     "./ruleset/proxy.yaml",
			Interval: 86400,
		},
		"proxyip": {
			Type:     "http",
			Behavior: "ipcidr",
			URL:      fmt.Sprintf("%s/rules/proxyip", origin),
			Path:     "./ruleset/proxyip.yaml",
			Interval: 86400,
		},
		"direct": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/direct", origin),
			Path:     "./ruleset/direct.yaml",
			Interval: 86400,
		},
		"directip": {
			Type:     "http",
			Behavior: "ipcidr",
			URL:      fmt.Sprintf("%s/rules/directip", origin),
			Path:     "./ruleset/directip.yaml",
			Interval: 86400,
		},
	}
}

// 生成规则（合并自定义规则和固定规则）
func (s *ClashService) generateRules(customRules string) []string {
	var rules []string

	// Cloudflare IP 直连（固定规则）
	rules = append(rules,
		"IP-CIDR,104.21.16.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.48.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.112.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.32.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.96.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.64.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.80.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.4.71/32,DIRECT,no-resolve",
		"IP-CIDR,172.67.131.193/32,DIRECT,no-resolve",
	)

	// 添加自定义规则
	if customRules != "" {
		lines := strings.Split(customRules, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				rules = append(rules, line)
			}
		}
	}

	// 添加固定的基础规则
	rules = append(rules,
		"DOMAIN-SUFFIX,ip6-localhost,DIRECT",
		"DOMAIN-SUFFIX,ip6-loopback,DIRECT",
		"DOMAIN-SUFFIX,lan,DIRECT",
		"DOMAIN-SUFFIX,local,DIRECT",
		"DOMAIN-SUFFIX,localhost,DIRECT",
		"IP-CIDR,0.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,198.18.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,224.0.0.0/4,DIRECT,no-resolve",
		"IP-CIDR6,::1/128,DIRECT,no-resolve",
		"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
		"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
		"IP-CIDR6,fd00::/8,DIRECT,no-resolve",
		"RULE-SET,proxyip,🚀 手动切换",
		"RULE-SET,proxy,🚀 手动切换",
		"RULE-SET,directip,DIRECT",
		"RULE-SET,direct,DIRECT",
		"GEOIP,LAN,DIRECT",
		"GEOIP,CN,DIRECT",
		"MATCH,🎯 兜底规则",
	)

	return rules
}

// 获取规则
func (s *ClashService) GetRules(ruleType string) (string, error) {
	// 检查缓存
	s.ruleCache.mu.RLock()
	if cached, ok := s.ruleCache.cache[ruleType]; ok {
		if time.Since(cached.UpdatedAt) < 24*time.Hour {
			s.ruleCache.mu.RUnlock()
			return cached.Content, nil
		}
	}
	s.ruleCache.mu.RUnlock()

	// 获取规则 URL
	urls := s.getRuleURLs(ruleType)
	if len(urls) == 0 {
		return "", fmt.Errorf("未知的规则类型: %s", ruleType)
	}

	// 获取并合并规则
	content, err := s.fetchAndMergeRules(urls)
	if err != nil {
		return "", err
	}

	// 缓存
	s.ruleCache.mu.Lock()
	s.ruleCache.cache[ruleType] = &CachedRule{
		Content:   content,
		UpdatedAt: time.Now(),
	}
	s.ruleCache.mu.Unlock()

	return content, nil
}

// 获取规则 URL
func (s *ClashService) getRuleURLs(ruleType string) []string {
	urlGroups := map[string][]string{
		"proxy": {
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/gfw.txt",
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/proxy.txt",
		},
		"direct": {
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/direct.txt",
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/private.txt",
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/tld-not-cn.txt",
		},
		"directip": {
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/lancidr.txt",
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/cncidr.txt",
		},
		"proxyip": {
			"https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/telegramcidr.txt",
		},
	}

	return urlGroups[ruleType]
}

// 获取并合并规则
func (s *ClashService) fetchAndMergeRules(urls []string) (string, error) {
	var contents []string

	for i, url := range urls {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		lines := strings.Split(string(body), "\n")

		// 第一个文件保留所有行，后续文件跳过第一行（标题）
		if i != 0 && len(lines) > 0 {
			lines = lines[1:]
		}

		// 过滤空行
		var filtered []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				filtered = append(filtered, line)
			}
		}

		contents = append(contents, strings.Join(filtered, "\n"))
	}

	return strings.Join(contents, "\n"), nil
}
