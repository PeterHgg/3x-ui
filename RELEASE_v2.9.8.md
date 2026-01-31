# Release v2.9.8-0verf1ow

## 🚀 核心优化

### ✅ Clash 订阅体验升级
- **重构低速分组**: 将"低速专线"重构为"低速单节点" (`🐢 低速单节点`)
- **全节点支持**: 该分组现在包含所有普通节点，不再使用单独的 xcdn 域名
- **手动选择模式**: 采用 `select` 模式，允许用户在自动负载均衡不理想时手动指定节点
- **健康检查**: 为该分组添加了 URL 延迟测试，直观展示节点连通性

## 📝 更新日志
- feat: refactor "Low Speed Line" to "Low Speed Single Node" group
- feat: add all proxies to low speed group with health check
- remove: deprecated xcdn generation logic
