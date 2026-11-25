package sub

import (
	"strings"
	"testing"
)

func TestClashDefaultRulesResolveChinaIPBeforeFallback(t *testing.T) {
	s := &ClashService{}
	rules := s.generateRules("DOMAIN,custom.example,DIRECT", "")
	wantTail := []string{
		"RULE-SET,google,🚀 手动切换",
		"RULE-SET,proxy,🚀 手动切换",
		"RULE-SET,direct,DIRECT",
		"RULE-SET,lancidr,DIRECT,no-resolve",
		"RULE-SET,telegramcidr,🚀 手动切换,no-resolve",
		"RULE-SET,cncidr,DIRECT",
		"GEOIP,LAN,DIRECT,no-resolve",
		"GEOIP,CN,DIRECT",
		"MATCH,🎯 兜底规则",
	}
	if len(rules) < len(wantTail) || strings.Join(rules[len(rules)-len(wantTail):], "\n") != strings.Join(wantTail, "\n") {
		t.Fatalf("unexpected default rule order or DNS resolution policy: %v", rules)
	}
	for _, rule := range rules {
		if strings.HasPrefix(rule, "IP-CIDR") && !strings.HasSuffix(rule, ",no-resolve") {
			t.Errorf("fixed CIDR exception must not trigger DNS: %s", rule)
		}
	}
	if rules[9] != "DOMAIN,custom.example,DIRECT" {
		t.Fatal("custom insertion changed")
	}
}

func TestClashFullRuleOverridePreserved(t *testing.T) {
	s := &ClashService{}
	rules := s.generateRules("DOMAIN,ignored.example,DIRECT", "# comment\nDOMAIN,kept.example,DIRECT\nMATCH,🚀 手动切换")
	if strings.Join(rules, "\n") != "DOMAIN,kept.example,DIRECT\nMATCH,🚀 手动切换" {
		t.Fatalf("full override was modified: %v", rules)
	}
}
