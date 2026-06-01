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
func (s *ClashService) GenerateClashConfig(uuid, password, cdnDomain string, count int, prefix, origin string, subPort int, customRules, customRuleProviders, fullRules string) (*ClashConfig, error) {
	var baseNodes []*model.Inbound

	if uuid != "" {
		baseNodes = s.findNodesByUUID(uuid)
	} else if password != "" {
		baseNodes = s.findNodesByPassword(password)
	}

	if len(baseNodes) == 0 {
		return nil, fmt.Errorf("未找到对应的节点")
	}

	settingService := new(service.SettingService)
	xcdnEnabled, _ := settingService.GetClashXcdnEnabled()

	// 生成 CDN 节点（按备注分组），如果开关开启，则额外生成 xcdn 节点
	proxiesMap, orderedGroupNames, xcdnProxyNames := s.generateCDNProxies(baseNodes, cdnDomain, count, prefix, subPort, uuid, password, xcdnEnabled)

	// 生成代理组
	proxyGroups := s.generateProxyGroups(proxiesMap, orderedGroupNames, xcdnProxyNames)

	// 生成规则提供者（可被自定义完整覆盖）
	ruleProviders := s.generateRuleProviders(origin, customRuleProviders)

	// 生成规则（支持完整覆盖或增量追加）
	rules := s.generateRules(customRules, fullRules)

	// 展平所有代理用于配置文件
	var allProxies []ClashProxy
	for _, ps := range proxiesMap {
		allProxies = append(allProxies, ps...)
	}

	return &ClashConfig{
		MixedPort:          7890,
		AllowLan:           true,
		Mode:               "rule",
		LogLevel:           "error",
		ExternalController: ":9090",
		UnifiedDelay:       true,
		TCPConcurrent:      true,
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

// 生成 CDN 节点，返回proxiesMap、按inbound ID排序的组名列表，以及 xcdn 节点名称列表
func (s *ClashService) generateCDNProxies(baseNodes []*model.Inbound, cdnDomain string, count int, prefix string, subPort int, targetUUID string, targetPassword string, xcdnEnabled bool) (map[string][]ClashProxy, []string, []string) {
	proxiesMap := make(map[string][]ClashProxy)
	groupIDMap := make(map[string]int) // 记录每个组名对应的最小inbound ID
	var xcdnProxyNames []string

	for _, inbound := range baseNodes {
		groupName := inbound.Remark
		if groupName == "" {
			groupName = "Default"
		}

		// 记录第一次出现的inbound ID（用于排序）
		if _, exists := groupIDMap[groupName]; !exists {
			groupIDMap[groupName] = inbound.Id
		}

		// 生成常规节点 (1cdn, 2cdn, ...)
		for i := 1; i <= count; i++ {
			cdnServer := fmt.Sprintf("%d%s.%s", i, prefix, cdnDomain)

			var proxy ClashProxy
			if inbound.Protocol == "vmess" {
				proxy = s.createVMessProxy(inbound, cdnServer, i, prefix, subPort, targetUUID)
			} else if inbound.Protocol == "trojan" {
				proxy = s.createTrojanProxy(inbound, cdnServer, i, prefix, subPort, targetPassword)
			}

			if proxy.Name != "" {
				proxiesMap[groupName] = append(proxiesMap[groupName], proxy)
			}
		}

		// 如果开启了三网优化节点选项，生成 xcdn 节点
		if xcdnEnabled {
			xcdnServer := fmt.Sprintf("x%s.%s", prefix, cdnDomain)
			var xcdnProxy ClashProxy
			if inbound.Protocol == "vmess" {
				xcdnProxy = s.createVMessProxyWithNamePrefix(inbound, xcdnServer, fmt.Sprintf("x%s", prefix), targetUUID)
			} else if inbound.Protocol == "trojan" {
				xcdnProxy = s.createTrojanProxyWithNamePrefix(inbound, xcdnServer, fmt.Sprintf("x%s", prefix), targetPassword)
			}

			if xcdnProxy.Name != "" {
				// 将名字改为主节点相同名字 + ⚡ 三网优选
				xcdnProxy.Name = groupName + " ⚡ 三网优选"

				// 放到负载均衡组内，并在稍后排到前面
				proxiesMap[groupName] = append([]ClashProxy{xcdnProxy}, proxiesMap[groupName]...)
				xcdnProxyNames = append(xcdnProxyNames, xcdnProxy.Name)
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

	return proxiesMap, orderedGroupNames, xcdnProxyNames
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
func (s *ClashService) createVMessProxy(inbound *model.Inbound, cdnServer string, index int, prefix string, subPort int, targetUUID string) ClashProxy {
	// 使用节点备注作为后缀
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%d%s%s", index, prefix, suffix)
	return s.createVMessProxyWithName(inbound, cdnServer, name, targetUUID)
}

// 创建特定名称前缀的 VMess 代理
func (s *ClashService) createVMessProxyWithNamePrefix(inbound *model.Inbound, cdnServer string, prefix string, targetUUID string) ClashProxy {
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%s%s", prefix, suffix)
	return s.createVMessProxyWithName(inbound, cdnServer, name, targetUUID)
}

// 内部创建 VMess 代理的基础逻辑
func (s *ClashService) createVMessProxyWithName(inbound *model.Inbound, cdnServer string, name string, targetUUID string) ClashProxy {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	clients, _ := settings["clients"].([]interface{})
	if len(clients) == 0 {
		return ClashProxy{}
	}

	var uuid string
	if targetUUID != "" {
		uuid = targetUUID
	} else {
		client, _ := clients[0].(map[string]interface{})
		uuid, _ = client["id"].(string)
	}

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
func (s *ClashService) createTrojanProxy(inbound *model.Inbound, cdnServer string, index int, prefix string, subPort int, targetPassword string) ClashProxy {
	// 使用节点备注作为后缀
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%d%s%s", index, prefix, suffix)
	return s.createTrojanProxyWithName(inbound, cdnServer, name, targetPassword)
}

// 创建特定名称前缀的 Trojan 代理
func (s *ClashService) createTrojanProxyWithNamePrefix(inbound *model.Inbound, cdnServer string, prefix string, targetPassword string) ClashProxy {
	suffix := ""
	if inbound.Remark != "" {
		suffix = "-" + inbound.Remark
	}
	name := fmt.Sprintf("%s%s", prefix, suffix)
	return s.createTrojanProxyWithName(inbound, cdnServer, name, targetPassword)
}

// 内部创建 Trojan 代理的基础逻辑
func (s *ClashService) createTrojanProxyWithName(inbound *model.Inbound, cdnServer string, name string, targetPassword string) ClashProxy {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	clients, _ := settings["clients"].([]interface{})
	if len(clients) == 0 {
		return ClashProxy{}
	}

	var password string
	if targetPassword != "" {
		password = targetPassword
	} else {
		client, _ := clients[0].(map[string]interface{})
		password, _ = client["password"].(string)
	}

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

// 生成代理组（使用按inbound ID排序的组名列表，并包含 xcdn 节点组）
func (s *ClashService) generateProxyGroups(proxiesMap map[string][]ClashProxy, orderedGroupNames []string, xcdnProxyNames []string) []ClashProxyGroup {
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

	// 如果有 xcdn 节点，把它们放在最前面
	if len(xcdnProxyNames) > 0 {
		topLevelProxies = append(topLevelProxies, xcdnProxyNames...)
	}

	// 2. 按排序后的顺序创建 load-balance 组
	for _, groupName := range orderedGroupNames {
		proxies, ok := proxiesMap[groupName]
		if !ok {
			continue
		}

		var proxyNames []string
		for _, p := range proxies {
			proxyNames = append(proxyNames, p.Name)
		}

		proxyGroupName := groupName
		if len(xcdnProxyNames) > 0 {
			proxyGroupName += " 🛟 稳定备用"
		}

		groups = append(groups, ClashProxyGroup{
			Name:     proxyGroupName,
			Type:     "load-balance",
			Proxies:  proxyNames,
			URL:      "http://cp.cloudflare.com/generate_204",
			Interval: 300,
			Lazy:     true,
			Strategy: "round-robin", // 显式设置为 round-robin
		})

		// load-balance 组加入手动切换
		topLevelProxies = append(topLevelProxies, proxyGroupName)
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

// 生成规则提供者（支持 JSON 完整覆盖）
func (s *ClashService) generateRuleProviders(origin string, customRuleProviders string) map[string]ClashRuleProvider {
	if strings.TrimSpace(customRuleProviders) != "" {
		custom := map[string]ClashRuleProvider{}
		if err := json.Unmarshal([]byte(customRuleProviders), &custom); err == nil && len(custom) > 0 {
			return custom
		}
	}
	return map[string]ClashRuleProvider{
		"reject": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/reject", origin),
			Path:     "./ruleset/reject.yaml",
			Interval: 86400,
		},
		"icloud": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/icloud", origin),
			Path:     "./ruleset/icloud.yaml",
			Interval: 86400,
		},
		"apple": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/apple", origin),
			Path:     "./ruleset/apple.yaml",
			Interval: 86400,
		},
		"google": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/google", origin),
			Path:     "./ruleset/google.yaml",
			Interval: 86400,
		},
		"proxy": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/proxy", origin),
			Path:     "./ruleset/proxy.yaml",
			Interval: 86400,
		},
		"direct": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/direct", origin),
			Path:     "./ruleset/direct.yaml",
			Interval: 86400,
		},
		"private": {
			Type:     "http",
			Behavior: "domain",
			URL:      fmt.Sprintf("%s/rules/private", origin),
			Path:     "./ruleset/private.yaml",
			Interval: 86400,
		},
		"telegramcidr": {
			Type:     "http",
			Behavior: "ipcidr",
			URL:      fmt.Sprintf("%s/rules/telegramcidr", origin),
			Path:     "./ruleset/telegramcidr.yaml",
			Interval: 86400,
		},
		"lancidr": {
			Type:     "http",
			Behavior: "ipcidr",
			URL:      fmt.Sprintf("%s/rules/lancidr", origin),
			Path:     "./ruleset/lancidr.yaml",
			Interval: 86400,
		},
		"applications": {
			Type:     "http",
			Behavior: "classical",
			URL:      fmt.Sprintf("%s/rules/applications", origin),
			Path:     "./ruleset/applications.yaml",
			Interval: 86400,
		},
	}
}

// 生成规则（支持完整覆盖或增量追加）
func (s *ClashService) generateRules(customRules string, fullRules string) []string {
	parseRules := func(raw string) []string {
		lines := strings.Split(raw, "\n")
		result := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				result = append(result, line)
			}
		}
		return result
	}

	if strings.TrimSpace(fullRules) != "" {
		return parseRules(fullRules)
	}

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

	rules = append(rules, parseRules(customRules)...)

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
		"RULE-SET,applications,DIRECT",
		"DOMAIN,clash.razord.top,DIRECT",
		"DOMAIN,yacd.haishan.me,DIRECT",
		"RULE-SET,private,DIRECT",
		"RULE-SET,reject,REJECT",
		"RULE-SET,icloud,DIRECT",
		"RULE-SET,apple,DIRECT",
		"DOMAIN-KEYWORD,douyin,DIRECT",
		"DOMAIN-KEYWORD,bytedance,DIRECT",
		"DOMAIN-KEYWORD,byteimg,DIRECT",
		"DOMAIN-KEYWORD,pstatp,DIRECT",
		"DOMAIN-KEYWORD,snssdk,DIRECT",
		"DOMAIN-KEYWORD,amemv,DIRECT",
		"RULE-SET,google,🚀 手动切换",
		"RULE-SET,proxy,🚀 手动切换",
		"RULE-SET,direct,DIRECT",
		"RULE-SET,lancidr,DIRECT",
		"RULE-SET,telegramcidr,🚀 手动切换",
		"GEOIP,LAN,DIRECT",
		"GEOIP,CN,DIRECT,no-resolve",
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
		"reject": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt",
		},
		"icloud": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/icloud.txt",
		},
		"apple": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/apple.txt",
		},
		"google": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/google.txt",
		},
		"proxy": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/proxy.txt",
		},
		"direct": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt",
		},
		"private": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/private.txt",
		},
		"telegramcidr": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/telegramcidr.txt",
		},
		"lancidr": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/lancidr.txt",
		},
		"applications": {
			"https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/applications.txt",
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
