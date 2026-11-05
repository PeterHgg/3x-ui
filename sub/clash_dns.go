package sub

import (
	"slices"
	"strings"
)

// 仅为通用域名规则集生成 DNS 策略，自定义域名只保留在 rules 中。
// 按实际规则顺序引用现有的 domain provider，避免自定义 providers 后出现悬空引用。
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
		if kind != "RULE-SET" {
			continue
		}
		switch value {
		case "private", "icloud", "apple", "google", "proxy", "direct":
		default:
			continue
		}
		provider, ok := providers[value]
		if !ok || provider.Behavior != "domain" {
			continue
		}
		domain := "rule-set:" + value
		if seen[domain] {
			continue
		}
		seen[domain] = true
		servers := proxyDNS
		if target == "DIRECT" {
			servers = []string{"system"}
		}
		// 仅合并相邻且同出口的规则集策略，不跨越其他 DNS 策略。
		if n := len(dns.NameserverPolicy); n > 0 {
			previous := &dns.NameserverPolicy[n-1]
			if slices.Equal(previous.Nameservers, servers) {
				previous.Domain += "," + strings.TrimPrefix(domain, "rule-set:")
				continue
			}
		}
		dns.NameserverPolicy = append(dns.NameserverPolicy, ClashDNSPolicy{
			Domain: domain, Nameservers: servers,
		})
	}
	return dns
}
