package sub

import (
	"strings"
	"testing"
)

func TestClashDefaultIPRulesDoNotResolve(t *testing.T) {
	s := &ClashService{}
	rules := s.generateRules("DOMAIN,custom.example,DIRECT", "")
	telegram, china := -1, -1
	for i, rule := range rules {
		if strings.HasPrefix(rule, "IP-CIDR") || strings.HasPrefix(rule, "GEOIP,") ||
			strings.HasPrefix(rule, "RULE-SET,lancidr,") || strings.HasPrefix(rule, "RULE-SET,cncidr,") ||
			strings.HasPrefix(rule, "RULE-SET,telegramcidr,") {
			if !strings.HasSuffix(rule, ",no-resolve") {
				t.Errorf("IP rule triggers DNS: %s", rule)
			}
		}
		if strings.HasPrefix(rule, "RULE-SET,telegramcidr,") {
			telegram = i
		}
		if strings.HasPrefix(rule, "RULE-SET,cncidr,") {
			china = i
		}
	}
	if telegram < 0 || china < 0 || telegram > china {
		t.Fatal("Telegram rule must precede country IP rules")
	}
	if rules[9] != "DOMAIN,custom.example,DIRECT" || rules[len(rules)-1] != "MATCH,🎯 兜底规则" {
		t.Fatal("custom insertion or final fallback changed")
	}
}

func TestClashFullRuleOverridePreserved(t *testing.T) {
	s := &ClashService{}
	rules := s.generateRules("DOMAIN,ignored.example,DIRECT", "# comment\nDOMAIN,kept.example,DIRECT\nMATCH,🚀 手动切换")
	if strings.Join(rules, "\n") != "DOMAIN,kept.example,DIRECT\nMATCH,🚀 手动切换" {
		t.Fatalf("full override was modified: %v", rules)
	}
}
