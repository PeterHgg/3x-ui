package sub

// 统一使用基础 DNS 配置，避免旧版 Stash 不支持策略数组、规则集引用
// 或 mihomo 专用的 DNS 出口字段。系统 DNS 不提供代理解析保证。
func generateClashDNS() ClashDNS {
	return ClashDNS{
		Enable:       true,
		EnhancedMode: "redir-host",
		Nameserver:   []string{"system"},
	}
}
