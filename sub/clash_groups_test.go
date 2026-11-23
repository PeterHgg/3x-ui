package sub

import (
	"slices"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestClashBackupGroupsAndYAML(t *testing.T) {
	for _, tc := range []struct {
		name string
		xcdn []string
	}{
		{name: "without xcdn"},
		{name: "with xcdn", xcdn: []string{"线路A ⚡ 三网优选"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := []string{"1cdn-线路A", "2cdn-线路A"}
			b := []string{"1cdn-线路B"}
			proxies := make(map[string][]ClashProxy)
			var allProxies []ClashProxy
			for i, names := range [][]string{append(slices.Clone(tc.xcdn), a...), b, {"线路C ⚡ 三网优选"}} {
				line := []string{"线路A", "线路B", "线路C"}[i]
				if line == "线路C" && len(tc.xcdn) == 0 {
					continue
				}
				for _, name := range names {
					proxy := ClashProxy{Name: name, Type: "socks5", Server: "127.0.0.1", Port: 9}
					proxies[line] = append(proxies[line], proxy)
					allProxies = append(allProxies, proxy)
				}
			}
			xcdn := slices.Clone(tc.xcdn)
			if len(xcdn) > 0 {
				xcdn = append(xcdn, "线路C ⚡ 三网优选")
			}
			s := &ClashService{}
			config := ClashConfig{
				Proxies:     allProxies,
				ProxyGroups: s.generateProxyGroups(proxies, []string{"线路A", "线路B", "线路C"}, xcdn),
				Rules:       []string{"MATCH,🎯 兜底规则"},
			}
			output := config.ToYAML()
			var fields map[string]any
			if err := yaml.Unmarshal([]byte(output), &fields); err != nil {
				t.Fatalf("invalid subscription YAML: %v", err)
			}
			if _, ok := fields["dns"]; ok {
				t.Fatal("subscription must not override client DNS")
			}
			var decoded ClashConfig
			if err := yaml.Unmarshal([]byte(output), &decoded); err != nil {
				t.Fatal(err)
			}
			backupNames := []string{"线路A 🛟 稳定备用1", "线路A 🛟 稳定备用2", "线路B 🛟 稳定备用1", "线路B 🛟 稳定备用2"}
			wantNames := append([]string{"🚀 手动切换"}, backupNames...)
			wantNames = append(wantNames, "🎯 兜底规则")
			var gotNames []string
			for _, group := range decoded.ProxyGroups {
				gotNames = append(gotNames, group.Name)
			}
			if !slices.Equal(gotNames, wantNames) {
				t.Fatalf("group order: got %v, want %v", gotNames, wantNames)
			}
			wantSelection := append(slices.Clone(xcdn), backupNames...)
			if !slices.Equal(decoded.ProxyGroups[0].Proxies, wantSelection) {
				t.Fatalf("manual selection order: got %v, want %v", decoded.ProxyGroups[0].Proxies, wantSelection)
			}
			for i, members := range [][]string{a, a, b, b} {
				group := decoded.ProxyGroups[i+1]
				wantType, wantStrategy := "url-test", ""
				if i%2 == 1 {
					wantType, wantStrategy = "load-balance", "round-robin"
				}
				if !slices.Equal(group.Proxies, members) || group.Type != wantType || group.Strategy != wantStrategy ||
					group.URL != "https://www.gstatic.com/generate_204" || group.Interval != 300 || !group.Lazy {
					t.Errorf("unexpected backup group: %+v", group)
				}
			}
			if !slices.Equal(decoded.ProxyGroups[len(wantNames)-1].Proxies, []string{"🚀 手动切换", "DIRECT"}) ||
				!slices.Equal(decoded.Rules, config.Rules) {
				t.Fatal("subscription fallback changed")
			}
		})
	}
}
