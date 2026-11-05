package sub

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestClashDNSDefaultsAndYAML(t *testing.T) {
	providers := map[string]ClashRuleProvider{
		"direct": {Behavior: "domain"},
		"google": {Behavior: "domain"},
	}
	rules := []string{
		"DOMAIN-SUFFIX,example.cn,DIRECT",
		"RULE-SET,google,🚀 手动切换",
		"RULE-SET,direct,DIRECT",
	}
	dns := generateClashDNS(rules, providers)
	if !dns.Enable || dns.EnhancedMode != "redir-host" || dns.RespectRules {
		t.Fatalf("unexpected DNS defaults: %+v", dns)
	}
	if len(dns.Fallback) != 0 || dns.FakeIPRange != "" {
		t.Fatal("must not enable fake-ip or local fallback")
	}
	for _, ns := range dns.Nameserver {
		if !strings.HasSuffix(ns, "#🚀 手动切换") {
			t.Fatalf("DNS is not pinned to proxy: %q", ns)
		}
	}
	config := ClashConfig{DNS: dns, Profile: ClashProfile{StoreSelected: true}}
	output := config.ToYAML()
	var decoded struct {
		DNS struct {
			Nameserver            []string            `yaml:"nameserver"`
			ProxyServerNameserver []string            `yaml:"proxy-server-nameserver"`
			DirectNameserver      []string            `yaml:"direct-nameserver"`
			Policy                map[string][]string `yaml:"nameserver-policy"`
		} `yaml:"dns"`
		Profile struct {
			StoreSelected bool `yaml:"store-selected"`
		} `yaml:"profile"`
	}
	if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, output)
	}
	if !decoded.Profile.StoreSelected || len(decoded.DNS.Nameserver) != 2 {
		t.Fatal("DNS serialization changed other top-level fields")
	}
	if len(decoded.DNS.ProxyServerNameserver) != 1 || decoded.DNS.ProxyServerNameserver[0] != "system" ||
		len(decoded.DNS.DirectNameserver) != 1 || decoded.DNS.DirectNameserver[0] != "system" {
		t.Fatal("node bootstrap and DIRECT must use system DNS")
	}
	if decoded.DNS.Policy["+.example.cn"][0] != "system" ||
		!strings.HasSuffix(decoded.DNS.Policy["rule-set:google"][0], "#🚀 手动切换") {
		t.Fatal("DNS policies did not survive YAML serialization")
	}
	if strings.Index(output, "'rule-set:google':") > strings.Index(output, "'rule-set:direct':") {
		t.Fatal("DNS policy order differs from rule order")
	}
}

func TestClashDNSMergesOnlyAdjacentCompatiblePolicies(t *testing.T) {
	providers := map[string]ClashRuleProvider{
		"private": {Behavior: "domain"},
		"icloud":  {Behavior: "domain"},
		"apple":   {Behavior: "domain"},
		"google":  {Behavior: "domain"},
		"proxy":   {Behavior: "domain"},
		"direct":  {Behavior: "domain"},
	}
	rules := []string{
		"DOMAIN-SUFFIX,first.example,DIRECT",
		"DOMAIN,second.example,DIRECT",
		"RULE-SET,private,DIRECT",
		"RULE-SET,icloud,DIRECT",
		"RULE-SET,apple,DIRECT",
		"RULE-SET,google,🚀 手动切换",
		"RULE-SET,proxy,🚀 手动切换",
		"RULE-SET,direct,DIRECT",
		"DOMAIN,local.example,DIRECT",
		"DOMAIN,override.local.example,🚀 手动切换",
		"DOMAIN,last.example,DIRECT",
	}
	dns := generateClashDNS(rules, providers)
	want := []string{
		"+.first.example,second.example",
		"rule-set:private,icloud,apple",
		"rule-set:google,proxy",
		"rule-set:direct",
		"local.example",
		"override.local.example",
		"last.example",
	}
	var got []string
	for _, policy := range dns.NameserverPolicy {
		got = append(got, policy.Domain)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged policies changed order or syntax: got %v, want %v", got, want)
	}
	output := (&ClashConfig{DNS: dns}).ToYAML()
	var decoded struct {
		DNS struct {
			Policy map[string][]string `yaml:"nameserver-policy"`
		} `yaml:"dns"`
	}
	if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range want {
		if len(decoded.DNS.Policy[key]) == 0 {
			t.Errorf("merged key missing from YAML: %s", key)
		}
	}
}

func TestClashDNSCustomProvidersAndFirstMatch(t *testing.T) {
	rules := []string{
		"DOMAIN-SUFFIX,steamstatic.com,🚀 手动切换",
		"DOMAIN-SUFFIX,steamstatic.com,DIRECT",
		"RULE-SET,removed,DIRECT",
		"RULE-SET,ip,DIRECT,no-resolve",
		"RULE-SET,applications,DIRECT",
		"DOMAIN,blocked.example,REJECT",
		"DOMAIN,local.example,DIRECT",
		"MATCH,🎯 兜底规则",
	}
	providers := map[string]ClashRuleProvider{
		"ip":           {Behavior: "ipcidr"},
		"applications": {Behavior: "classical"},
	}
	dns := generateClashDNS(rules, providers)
	if len(dns.NameserverPolicy) != 2 {
		t.Fatalf("unexpected policies: %+v", dns.NameserverPolicy)
	}
	if dns.NameserverPolicy[0].Domain != "+.steamstatic.com" ||
		!strings.HasSuffix(dns.NameserverPolicy[0].Nameservers[0], "#🚀 手动切换") {
		t.Fatal("duplicate rules must not override first match")
	}
	if dns.NameserverPolicy[1].Domain != "local.example" || dns.NameserverPolicy[1].Nameservers[0] != "system" {
		t.Fatal("custom DIRECT domain not reflected in DNS policy")
	}
}
