# Release v2.9.7-0verf1ow

## 🚀 核心优化

### ✅ Clash 订阅 DNS 增强
- **新增 DNS 配置**: 强制开启 `redir-host` 模式 DNS 解析
- **修复国内应用误判**: 解决平安好车主、银行等国内 App 因 fake-ip 导致 GEOIP 规则失效而被错误代理的问题
- **优化直连体验**: 国内域名现在能正确解析为真实 IP 并命中 CN 直连规则

## 📝 更新日志
- feat: add DNS config to Clash subscription (redir-host mode)
- fix: resolve issue where domestic apps were proxied due to fake-ip mode
