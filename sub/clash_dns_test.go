package sub

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestClashDNSCompatibilityDefaultsAndYAML(t *testing.T) {
	dns := generateClashDNS()
	if !dns.Enable || dns.EnhancedMode != "redir-host" || dns.IPv6 {
		t.Fatalf("unexpected DNS defaults: %+v", dns)
	}
	if !reflect.DeepEqual(dns.Nameserver, []string{"system"}) || len(dns.Fallback) != 0 || dns.FakeIPRange != "" {
		t.Fatal("compatibility DNS must use system without fake-ip or public fallback")
	}
	config := ClashConfig{DNS: dns, Profile: ClashProfile{StoreSelected: true, Interval: 12}}
	output := config.ToYAML()
	// 模拟旧版解析器的字符串型 policy，防止重新输出数组策略。
	var decoded struct {
		DNS struct {
			Enable       bool              `yaml:"enable"`
			IPv6         bool              `yaml:"ipv6"`
			EnhancedMode string            `yaml:"enhanced-mode"`
			Nameserver   []string          `yaml:"nameserver"`
			Policy       map[string]string `yaml:"nameserver-policy"`
		} `yaml:"dns"`
		Profile struct {
			StoreSelected bool `yaml:"store-selected"`
			Interval      int  `yaml:"interval"`
		} `yaml:"profile"`
	}
	if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("invalid compatibility YAML: %v\n%s", err, output)
	}
	if !decoded.DNS.Enable || decoded.DNS.EnhancedMode != "redir-host" ||
		!reflect.DeepEqual(decoded.DNS.Nameserver, []string{"system"}) || len(decoded.DNS.Policy) != 0 {
		t.Fatal("DNS serialization changed compatibility defaults")
	}
	if !decoded.Profile.StoreSelected || decoded.Profile.Interval != 12 {
		t.Fatal("DNS serialization changed profile settings")
	}
	for _, unsupported := range []string{
		"nameserver-policy:", "proxy-server-nameserver:", "direct-nameserver:",
		"direct-nameserver-follow-policy:", "respect-rules:", "follow-rule:",
		"#🚀", "dns-query", "fake-ip-range:", "tun:",
	} {
		if strings.Contains(output, unsupported) {
			t.Errorf("compatibility YAML contains unsupported or unintended setting: %s", unsupported)
		}
	}
}

func TestClashCompatibilityDNSPreservesRoutingAndNodes(t *testing.T) {
	s := &ClashService{}
	rules := s.generateRules("DOMAIN-SUFFIX,linux.do,🚀 手动切换", "")
	originalRules := append([]string(nil), rules...)
	config := ClashConfig{
		DNS: generateClashDNS(), Rules: rules,
		Proxies:       []ClashProxy{{Name: "test-node", Type: "socks5", Server: "127.0.0.1", Port: 9}},
		ProxyGroups:   []ClashProxyGroup{{Name: "🚀 手动切换", Type: "select", Proxies: []string{"test-node"}}},
		RuleProviders: s.generateRuleProviders("https://example.invalid", ""),
	}
	var decoded struct {
		Rules         []string                     `yaml:"rules"`
		Proxies       []ClashProxy                 `yaml:"proxies"`
		ProxyGroups   []ClashProxyGroup            `yaml:"proxy-groups"`
		RuleProviders map[string]ClashRuleProvider `yaml:"rule-providers"`
	}
	if err := yaml.Unmarshal([]byte(config.ToYAML()), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Rules, originalRules) || !reflect.DeepEqual(rules, originalRules) {
		t.Fatal("compatibility DNS changed routing rules")
	}
	if !reflect.DeepEqual(decoded.Proxies, config.Proxies) ||
		!reflect.DeepEqual(decoded.ProxyGroups, config.ProxyGroups) ||
		!reflect.DeepEqual(decoded.RuleProviders, config.RuleProviders) {
		t.Fatal("compatibility DNS changed nodes, groups or rule providers")
	}
}
