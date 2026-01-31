# Release v2.9.9-0verf1ow

## 🛠 代码清理

### 🧹 移除废弃逻辑
- **移除开关**: 彻底移除了已弃用的 `lowSpeedLine` 开关参数
- **默认启用**: "低速单节点" (`🐢 低速单节点`) 分组现在作为核心功能默认集成，无需配置

此版本包含 v2.9.8 的所有功能特性：
- 重构低速分组为手动选择模式 (`select`)
- 包含所有节点，移除 xcdn 生成逻辑
- 添加 URL 健康检查

## 📝 更新日志
- refactor: remove deprecated lowSpeedLine parameter and toggle logic
- chore: cleanup unused code in clash subscription generation
