package sub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type ClashService struct {
	ruleCache *RuleCache
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
	}
}

// 生成 Clash 配置
func (s *ClashService) GenerateClashConfig(uuid, password, cdnDomain string, count int, prefix, origin string, subPort int) (*ClashConfig, error) {
	var baseNodes []*model.Inbound

	if uuid != "" {
		baseNodes = s.findNodesByUUID(uuid)
	} else if password != "" {
		baseNodes = s.findNodesByPassword(password)
	}

	if len(baseNodes) == 0 {
		return nil, fmt.Errorf("未找到对应的节点")
	}

	// 生成 CDN 节点
	proxies := s.generateCDNProxies(baseNodes, cdnDomain, count, prefix, subPort)

	// 生成代理组
	proxyGroups := s.generateProxyGroups(proxies)

	// 生成规则提供者
	ruleProviders := s.generateRuleProviders(origin)

	// 生成固定规则
	rules := s.generateRules()

	return &ClashConfig{
		MixedPort:          7890,
		AllowLan:           true,
		Mode:               "rule",
		LogLevel:           "info",
		ExternalController: ":9090",
		UnifiedDelay:       true,
		TCPConcurrent:      true,
		Proxies:            proxies,
		ProxyGroups:        proxyGroups,
		RuleProviders:      ruleProviders,
		Rules:              rules,
	}, nil
}

// 根据 UUID 查找节点
func (s *ClashService) findNodesByUUID(uuid string) []*model.Inbound {
	db := database.GetDB()
	var allInbounds []*model.Inbound
	db.Where("protocol = ?", "vmess").Find(&allInbounds)

	var result []*model.Inbound
	for _, inbound := range allInbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}

		if clients, ok := settings["clients"].([]interface{}); ok {
			for _, client := range clients {
				if c, ok := client.(map[string]interface{}); ok {
					if c["id"] == uuid {
						result = append(result, inbound)
						break
					}
				}
			}
		}
	}

	return result
}

// 根据密码查找节点
func (s *ClashService) findNodesByPassword(password string) []*model.Inbound {
	db := database.GetDB()
	var allInbounds []*model.Inbound
	db.Where("protocol = ?", "trojan").Find(&allInbounds)

	var result []*model.Inbound
	for _, inbound := range allInbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}

		if clients, ok := settings["clients"].([]interface{}); ok {
			for _, client := range clients {
				if c, ok := client.(map[string]interface{}); ok {
					if c["password"] == password {
						result = append(result, inbound)
						break
					}
				}
			}
		}
	}

	return result
}

// 识别节点类型
func (s *ClashService) identifyNodeType(inbound *model.Inbound) string {
	remark := strings.ToUpper(inbound.Remark)

	if strings.Contains(remark, "RN") {
		return NodeTypeRN
	}
	if strings.Contains(remark, "SC") {
		return NodeTypeSC
	}
	if strings.Contains(remark, "WARP") || strings.Contains(remark, "CF") {
		return NodeTypeWARP
	}

	return NodeTypeDefault
}

// 生成 CDN 节点
func (s *ClashService) generateCDNProxies(baseNodes []*model.Inbound, cdnDomain string, count int, prefix string, subPort int) []ClashProxy {
	var proxies []ClashProxy

	for _, inbound := range baseNodes {
		nodeType := s.identifyNodeType(inbound)

		for i := 1; i <= count; i++ {
			cdnServer := fmt.Sprintf("%d%s.%s", i, prefix, cdnDomain)

			var proxy ClashProxy
			if inbound.Protocol == "vmess" {
				proxy = s.createVMessProxy(inbound, cdnServer, nodeType, i, prefix, subPort)
			} else if inbound.Protocol == "trojan" {
				proxy = s.createTrojanProxy(inbound, cdnServer, nodeType, i, prefix, subPort)
			}

			if proxy.Name != "" {
				proxies = append(proxies, proxy)
			}
		}
	}

	return proxies
}

// 创建 VMess 代理
func (s *ClashService) createVMessProxy(inbound *model.Inbound, cdnServer, nodeType string, index int, prefix string, subPort int) ClashProxy {
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
			Path: GetPathForType(nodeType),
		},
	}
}

// 创建 Trojan 代理
func (s *ClashService) createTrojanProxy(inbound *model.Inbound, cdnServer, nodeType string, index int, prefix string, subPort int) ClashProxy {
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
			Path: GetPathForType(nodeType),
		},
	}
}

// 生成代理组
func (s *ClashService) generateProxyGroups(proxies []ClashProxy) []ClashProxyGroup {
	// 按后缀分类节点
	groupMap := make(map[string][]string)
	groupOrder := []string{} // 保持顺序

	for _, proxy := range proxies {
		// 提取后缀（如 -RN, -SC, -WARP）
		parts := strings.Split(proxy.Name, "-")
		var groupKey string
		if len(parts) > 1 {
			groupKey = parts[len(parts)-1] // 最后一部分作为分组key
		} else {
			groupKey = "Default" // 没有后缀的归为Default
		}

		if _, exists := groupMap[groupKey]; !exists {
			groupOrder = append(groupOrder, groupKey)
		}
		groupMap[groupKey] = append(groupMap[groupKey], proxy.Name)
	}

	// 创建代理组
	groups := []ClashProxyGroup{}

	// 1. 创建顶层 select 组，包含所有 load-balance 组
	loadBalanceGroupNames := []string{}
	for _, key := range groupOrder {
		// 默认组名就是后缀名，后续可以从设置中读取自定义名称
		groupName := key
		loadBalanceGroupNames = append(loadBalanceGroupNames, groupName)
	}

	groups = append(groups, ClashProxyGroup{
		Name:     "🚀 手动切换",
		Type:     "select",
		Proxies:  loadBalanceGroupNames,
		URL:      "http://cp.cloudflare.com/generate_204",
		Interval: 300,
	})

	// 2. 为每个分组创建 load-balance 组
	for _, key := range groupOrder {
		groupName := key // 默认名称
		nodes := groupMap[key]

		groups = append(groups, ClashProxyGroup{
			Name:     groupName,
			Type:     "load-balance",
			Proxies:  nodes,
			URL:      "http://cp.cloudflare.com/generate_204",
			Interval: 300,
			Strategy: "round-robin",
		})
	}

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

// 生成固定规则
func (s *ClashService) generateRules() []string {
	return []string{
		"IP-CIDR,104.21.16.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.48.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.112.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.32.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.96.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.64.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.80.1/32,DIRECT,no-resolve",
		"IP-CIDR,104.21.4.71/32,DIRECT,no-resolve",
		"IP-CIDR,172.67.131.193/32,DIRECT,no-resolve",
		"DOMAIN-SUFFIX,szbdyd.com,REJECT",
		"DOMAIN-SUFFIX,mcdn.bilivideo.com,REJECT",
		"DOMAIN-SUFFIX,mcdn.bilivideo.cn,REJECT",
		"DOMAIN-SUFFIX,edge.mountaintoys.cn,REJECT",
		"DOMAIN-SUFFIX,scaleway.com,DIRECT",
		"DOMAIN-SUFFIX,linux.do,🚀 手动切换",
		"DOMAIN-SUFFIX,epicgames.com,DIRECT",
		"DOMAIN-SUFFIX,epicgames.dev,DIRECT",
		"DOMAIN-SUFFIX,epicgames.net,DIRECT",
		"DOMAIN-SUFFIX,unrealengine.com,DIRECT",
		"DOMAIN,steamcdn-a.akamaihd.net,DIRECT",
		"DOMAIN-SUFFIX,cm.steampowered.com,DIRECT",
		"DOMAIN-SUFFIX,steamserver.net,DIRECT",
		"DOMAIN,steam-chat.com,🚀 手动切换",
		"DOMAIN-SUFFIX,steamstatic.com,🚀 手动切换",
		"DOMAIN,api.steampowered.com,🚀 手动切换",
		"DOMAIN,store.steampowered.com,🚀 手动切换",
		"DOMAIN-SUFFIX,steamcommunity.com,🚀 手动切换",
		"DOMAIN-SUFFIX,steamgames.com,DIRECT",
		"DOMAIN-SUFFIX,steamusercontent.com,DIRECT",
		"DOMAIN-SUFFIX,steamcontent.com,🚀 手动切换",
		"DOMAIN-SUFFIX,steamstatic.com,DIRECT",
		"DOMAIN-SUFFIX,steamcdn-a.akamaihd.net,DIRECT",
		"DOMAIN-SUFFIX,steamstat.us,DIRECT",
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
		"MATCH,🚀 手动切换",
	}
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
