package sub

import (
	"fmt"
	"strings"
)

// Clash 配置结构体
type ClashConfig struct {
	MixedPort          int                          `yaml:"mixed-port"`
	AllowLan           bool                         `yaml:"allow-lan"`
	Mode               string                       `yaml:"mode"`
	LogLevel           string                       `yaml:"log-level"`
	ExternalController string                       `yaml:"external-controller"`
	UnifiedDelay       bool                         `yaml:"unified-delay"`
	TCPConcurrent      bool                         `yaml:"tcp-concurrent"`
	Profile            ClashProfile                 `yaml:"profile,omitempty"`
	DNS                ClashDNS                     `yaml:"dns,omitempty"`
	Proxies            []ClashProxy                 `yaml:"proxies"`
	ProxyGroups        []ClashProxyGroup            `yaml:"proxy-groups"`
	RuleProviders      map[string]ClashRuleProvider `yaml:"rule-providers"`
	Rules              []string                     `yaml:"rules"`
}

// Clash Profile 配置
type ClashProfile struct {
	StoreSelected bool `yaml:"store-selected,omitempty"` // 存储选择的节点
	Tracing       bool `yaml:"tracing,omitempty"`        // 追踪模式
	Interval      int  `yaml:"interval,omitempty"`       // 自动更新间隔（小时）
}

// DNS 配置
type ClashDNS struct {
	Enable       bool     `yaml:"enable"`
	EnhancedMode string   `yaml:"enhanced-mode"`
	Nameserver   []string `yaml:"nameserver"`
}

// Clash 代理节点
type ClashProxy struct {
	Name           string          `yaml:"name"`
	Type           string          `yaml:"type"` // vmess, trojan
	Server         string          `yaml:"server"`
	Port           int             `yaml:"port"`
	UUID           string          `yaml:"uuid,omitempty"`
	AlterID        int             `yaml:"alterId,omitempty"`
	Cipher         string          `yaml:"cipher,omitempty"`
	Password       string          `yaml:"password,omitempty"`
	UDP            bool            `yaml:"udp"`
	TLS            bool            `yaml:"tls,omitempty"`
	SkipCertVerify bool            `yaml:"skip-cert-verify,omitempty"`
	Network        string          `yaml:"network,omitempty"`
	WSOptions      *ClashWSOptions `yaml:"ws-opts,omitempty"`
}

// WebSocket 选项
type ClashWSOptions struct {
	Path string `yaml:"path"`
}

// 代理组
type ClashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"` // select, load-balance
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
	Strategy string   `yaml:"strategy,omitempty"` // round-robin
}

// 规则提供者
type ClashRuleProvider struct {
	Type     string `yaml:"type"`     // http
	Behavior string `yaml:"behavior"` // domain, ipcidr
	URL      string `yaml:"url"`
	Path     string `yaml:"path"`
	Interval int    `yaml:"interval"`
}

// 节点类型
const (
	NodeTypeDefault = "default" // 路径 /
	NodeTypeRN      = "rn"      // 路径 /rn
	NodeTypeSC      = "sc"      // 路径 /sc
	NodeTypeWARP    = "cf"      // 路径 /cf
)

// 获取节点后缀
func GetSuffixForType(nodeType string) string {
	suffixes := map[string]string{
		NodeTypeDefault: "",
		NodeTypeRN:      "-RN",
		NodeTypeSC:      "-SC",
		NodeTypeWARP:    "-WARP",
	}
	if suffix, ok := suffixes[nodeType]; ok {
		return suffix
	}
	return ""
}

// ToYAML 将 ClashConfig 转换为 YAML 字符串
func (c *ClashConfig) ToYAML() string {
	var sb strings.Builder

	// 基础配置
	sb.WriteString(fmt.Sprintf("mixed-port: %d\n", c.MixedPort))
	sb.WriteString(fmt.Sprintf("allow-lan: %t\n", c.AllowLan))
	sb.WriteString(fmt.Sprintf("mode: %s\n", c.Mode))
	sb.WriteString(fmt.Sprintf("log-level: %s\n", c.LogLevel))
	sb.WriteString(fmt.Sprintf("external-controller: '%s'\n", c.ExternalController))
	sb.WriteString(fmt.Sprintf("unified-delay: %t\n", c.UnifiedDelay))
	sb.WriteString(fmt.Sprintf("tcp-concurrent: %t\n", c.TCPConcurrent))

	// DNS 配置
	if c.DNS.Enable {
		sb.WriteString("\ndns:\n")
		sb.WriteString("  enable: true\n")
		sb.WriteString(fmt.Sprintf("  enhanced-mode: %s\n", c.DNS.EnhancedMode))
		sb.WriteString("  nameserver:\n")
		for _, ns := range c.DNS.Nameserver {
			sb.WriteString(fmt.Sprintf("    - %s\n", ns))
		}
	}

	// Profile配置（自动更新间隔等）
	if c.Profile.StoreSelected || c.Profile.Tracing || c.Profile.Interval > 0 {
		sb.WriteString("profile:\n")
		if c.Profile.StoreSelected {
			sb.WriteString("  store-selected: true\n")
		}
		if c.Profile.Tracing {
			sb.WriteString("  tracing: true\n")
		}
		if c.Profile.Interval > 0 {
			sb.WriteString(fmt.Sprintf("  interval: %d\n", c.Profile.Interval))
		}
	}
	sb.WriteString("\n")

	// Proxies
	sb.WriteString("proxies:\n")
	for _, proxy := range c.Proxies {
		sb.WriteString(fmt.Sprintf("    - { name: %s, type: %s, server: %s, port: %d",
			proxy.Name, proxy.Type, proxy.Server, proxy.Port))

		if proxy.UUID != "" {
			sb.WriteString(fmt.Sprintf(", uuid: %s, alterId: %d, cipher: %s",
				proxy.UUID, proxy.AlterID, proxy.Cipher))
		}
		if proxy.Password != "" {
			sb.WriteString(fmt.Sprintf(", password: %s", proxy.Password))
		}
		if proxy.SkipCertVerify {
			sb.WriteString(", skip-cert-verify: true")
		}

		sb.WriteString(fmt.Sprintf(", udp: %t", proxy.UDP))

		if proxy.TLS {
			sb.WriteString(", tls: true")
		}
		if proxy.Network != "" {
			sb.WriteString(fmt.Sprintf(", network: %s", proxy.Network))
		}
		if proxy.WSOptions != nil {
			sb.WriteString(fmt.Sprintf(", ws-opts: { path: \"%s\" }", proxy.WSOptions.Path))
		}

		sb.WriteString(" }\n")
	}

	// Proxy Groups
	sb.WriteString("\nproxy-groups:\n")
	for _, group := range c.ProxyGroups {
		sb.WriteString(fmt.Sprintf("    - { name: %s, type: %s", group.Name, group.Type))

		if len(group.Proxies) > 0 {
			sb.WriteString(", proxies: [")
			for i, proxy := range group.Proxies {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(proxy)
			}
			sb.WriteString("]")
		}

		if group.URL != "" {
			sb.WriteString(fmt.Sprintf(", url: '%s'", group.URL))
		}
		if group.Interval > 0 {
			sb.WriteString(fmt.Sprintf(", interval: %d", group.Interval))
		}
		if group.Strategy != "" {
			sb.WriteString(fmt.Sprintf(", strategy: %s", group.Strategy))
		}

		sb.WriteString(" }\n")
	}

	// Rule Providers
	sb.WriteString("\nrule-providers:\n")
	for name, provider := range c.RuleProviders {
		sb.WriteString(fmt.Sprintf("    %s: { type: %s, behavior: %s, url: '%s', path: %s, interval: %d }\n",
			name, provider.Type, provider.Behavior, provider.URL, provider.Path, provider.Interval))
	}

	// Rules
	sb.WriteString("\nrules:\n")
	for _, rule := range c.Rules {
		sb.WriteString(fmt.Sprintf("    - '%s'\n", rule))
	}

	return sb.String()
}
