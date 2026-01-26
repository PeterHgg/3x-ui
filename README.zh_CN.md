[English](/README.en.md) | [中文](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/PeterHgg/3x-ui.svg)](https://github.com/PeterHgg/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/PeterHgg/3x-ui/release.yml.svg)](https://github.com/PeterHgg/3x-ui/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/PeterHgg/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/PeterHgg/3x-ui/total.svg)](https://github.com/PeterHgg/3x-ui/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

**3X-UI** — 一个基于网页的高级开源控制面板，专为管理 Xray-core 服务器而设计。它提供了用户友好的界面，用于配置和监控各种 VPN 和代理协议。

> [!IMPORTANT]
> 本项目仅用于个人学习和通信，请勿将其用于非法目的，请勿在生产环境中使用。

作为原始 X-UI 项目的增强版本，3X-UI 提供了更好的稳定性、更广泛的协议支持和额外的功能。

## 特色功能

### 主从节点同步 (v2.8.x+)
- ✅ **自动客户端同步**：主节点的客户端变更自动同步到所有从节点
- ✅ **流量聚合统计**：从节点流量自动累加到主节点，统一配额管理
- ✅ **只读从节点**：从节点自动禁用客户端编辑，防止数据不一致
- ✅ **定期同步恢复**：每5分钟自动检查并恢复同步，确保数据一致性
- ✅ **实时同步提示**：设置同步源和添加客户端时显示同步进度

### Clash 定制订阅 (v2.8.x+)
- ✅ **智能节点生成**：自动批量生成指向 CF 的 CDN 节点
- ✅ **备注分组**：根据入站备注自动智能分组并启用负载均衡
- ✅ **路径同步**：自动读取入站 WebSocket 路径配置
- ✅ **端口适配**：强制使用 443 端口，完美适配 Cloudflare CDN
- ✅ **自定义规则**：编辑器自定义 Clash 规则，支持注释 (v2.8.55+)
- ✅ **流量信息显示**：支持实时显示已用/总流量用量 (v2.8.56+)
- ✅ **订阅特征配置**：自动设置订阅名 (Email) 及自动更新间隔 (v2.8.56+)

## 快速开始

```bash
bash <(curl -Ls https://raw.githubusercontent.com/PeterHgg/3x-ui/main/install.sh)
```

## 版本说明

当前版本：**v2.8.76-0verf1ow**

- 基于 MHSanaei/3x-ui 项目
- 由 **0verf1ow** 维护和增强
- 增强了订阅流量信息与负载均衡策略

## 特别感谢

- [MHSanaei](https://github.com/MHSanaei/)
- [alireza0](https://github.com/alireza0/)

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat)

## 支持项目

**如果这个项目对您有帮助，您可以给它一个**:star2:

## 星标历史

[![Stargazers over time](https://starchart.cc/PeterHgg/3x-ui.svg?variant=adaptive)](https://starchart.cc/PeterHgg/3x-ui)
