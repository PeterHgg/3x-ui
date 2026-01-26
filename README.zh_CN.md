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
- ✅ **全自动同步**：客户端实时同步至从节点，支持断连定期自动恢复，保障数据一致性
- ✅ **统一流量管理**：聚合所有节点流量统计，统一进行配额管理与监控
- ✅ **数据安全保障**：从节点强制只读模式，操作时提供实时同步状态提示

### Clash 定制订阅 (v2.8.x+)
- ✅ **Cloudflare 深度适配**：自动生成 CDN 节点，强制 443 端口，支持入站备注智能分组
- ✅ **自动化配置**：自动同步 WS 路径，支持隐藏端口及自动设置订阅信息（名称/更新间隔/流量）
- ✅ **高级规则系统**：内置可视化规则编辑器，集成 Loyalsoldier/clash-rules 自动代理

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
