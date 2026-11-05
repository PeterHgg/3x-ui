package sub

import "strings"

// 根据实际域名规则选择解析出口，保留完整覆盖和自定义规则的优先级。
// 只引用现有的 domain provider，避免自定义 providers 后出现悬空引用。
func generateClashDNS(rules []string, providers map[string]ClashRuleProvider) ClashDNS {
	proxyDNS := []string{
		"https://1.1.1.1/dns-query#🚀 手动切换",
		"https://8.8.8.8/dns-query#🚀 手动切换",
	}
	dns := ClashDNS{
		Enable:                true,
		EnhancedMode:          "redir-host",
		Nameserver:            proxyDNS,
		ProxyServerNameserver: []string{"system"},
		DirectNameserver:      []string{"system"},
	}
	seen := make(map[string]bool)
	for _, rule := range rules {
		parts := strings.Split(rule, ",")
		if len(parts) < 3 {
			continue
		}
		kind := strings.ToUpper(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		target := strings.TrimSpace(parts[2])
		if target != "DIRECT" && target != "🚀 手动切换" {
			continue
		}
		var domain string
		switch kind {
		case "DOMAIN":
			domain = strings.ToLower(value)
		case "DOMAIN-SUFFIX":
			domain = "+." + strings.ToLower(value)
		case "RULE-SET":
			provider, ok := providers[value]
			if !ok || provider.Behavior != "domain" {
				continue
			}
			domain = "rule-set:" + value
		default:
			continue
		}
		if seen[domain] {
			continue
		}
		seen[domain] = true
		servers := proxyDNS
		if target == "DIRECT" {
			servers = []string{"system"}
		}
		dns.NameserverPolicy = append(dns.NameserverPolicy, ClashDNSPolicy{
			Domain: domain, Nameservers: servers,
		})
	}
	return dns
}
