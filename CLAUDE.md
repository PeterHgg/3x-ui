# 项目规范

## 开发流程
- 每次代码修改完成后，必须进行 git 提交。
- 提交后，应当创建对应的 Release。如果 `gh release create` 失败（如权限报错），应手动创建 git tag 并推送（`git tag vX.X.X && git push origin vX.X.X`）以触发自动化构建。

## 语言偏好
- 所有的沟通、回复、文档说明请使用简体中文。

## 协作方式
- 你是运维而非程序员，不熟悉 diff/review；需要我用易懂的说明代你执行相关步骤。
- 稳定构建流程（已验证可触发构建）：
  1) git add <改动文件>
  2) git commit -m "..."
  3) git push origin main
  4) git tag vX.X.X
  5) git push origin vX.X.X
  6) 等待 GitHub Actions 触发构建（无需创建 Release）
- 版本号规范：构建脚本写入的版本号不带前缀 v（页面模板会自动加 v）。

## 新增功能总结 (对比 MHSanaei/3x-ui 原版)

### 1. 主从节点同步系统 (Master-Slave Sync)
- **多节点管理**：支持将入站 (Inbound) 设置为“从节点”，通过 `SyncSourceId` 关联主节点。
- **自动同步**：包含 `PeriodicSyncJob` 定时任务，通过 SHA256 哈希校验确保主从节点用户配置一致。
- **流量汇总**：支持跨节点流量统计，从节点可实时从主节点同步用户限额、到期时间和启用状态。
- **实时触发**：主节点修改用户后，会立即触发 `TriggerSyncToSlaves` 向所有子节点推送更新。

### 2. 增强型订阅系统 (Subscription V2)
- **Clash 深度支持**：新增 `sub/clash.go` 和 `sub/clashService.go`，提供更强大的 Clash 配置文件生成能力。
- **JSON 订阅**：优化了订阅的 JSON 输出逻辑，支持更灵活的客户端适配。
- **独立设置页**：在面板设置中新增了专门的订阅管理通用配置页面。

### 3. 诊断与日志优化
- **深度诊断日志**：引入了更详细的诊断日志记录 (Diagnostic Logs)，便于排查主从同步和流量统计问题。
- **匹配逻辑优化**：优化了主从模式下的流量匹配和归因逻辑，减少统计误差。

### 4. 界面与部署增强
- **全新登录页**：大幅重构了登录界面逻辑与 UI。
- **自动化发布**：新增 `push-and-release.ps1` 脚本，支持自动化的 GitHub Release 发布流程。
- **部署报告**：维护了详尽的 `DEPLOYMENT_REPORT.md`，记录版本演进与部署细节。

### 5. 其他改进
- **客户端管理**：新增 `web/service/client.go` 专门处理客户端逻辑。
- **IP 审计增强**：改进了 `check_client_ip_job.go` 的 IP 连接数限制检查逻辑。
