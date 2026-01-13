[中文](/README.md) | [English](/README.en.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

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

**3X-UI** — 先进的开源 Web 管理面板，专为管理 Xray-core 服务器而设计。它提供了用户友好的界面，用于配置和监控各种 VPN 和代理协议。

> [!IMPORTANT]
> 本项目仅供个人学习使用，请勿用于非法用途，也请勿在生产环境中使用。

作为原始 X-UI 项目的增强分支，3X-UI 提供了改进的稳定性、更广泛的协议支持和额外的功能。

## 特色功能

### 主从节点同步 (v2.8.x+)
- ✅ **自动客户端同步**：主节点的客户端变更自动同步到所有从节点
- ✅ **流量聚合统计**：从节点流量自动累加到主节点，统一配额管理
- ✅ **只读从节点**：从节点自动禁用客户端编辑，防止数据不一致
- ✅ **定期同步恢复**：每5分钟自动检查并恢复同步，确保数据一致性
- ✅ **实时同步提示**：设置同步源和添加客户端时显示同步进度

## 快速开始

### 一键安装

```bash
bash <(curl -Ls https://raw.githubusercontent.com/PeterHgg/3x-ui/main/install.sh)
```

### 一键更新

```bash
bash <(curl -Ls https://raw.githubusercontent.com/PeterHgg/3x-ui/main/install.sh)
```

### 管理脚本

安装后，使用以下命令管理面板：

```bash
x-ui              # 显示管理菜单
x-ui start        # 启动面板
x-ui stop         # 停止面板
x-ui restart      # 重启面板
x-ui status       # 查看状态
x-ui enable       # 设置开机自启
x-ui disable      # 取消开机自启
x-ui log          # 查看日志
x-ui update       # 更新面板
x-ui install      # 安装面板
x-ui uninstall    # 卸载面板
```

## 版本说明

当前版本：**v2.8.23-0verf1ow**

- 基于 MHSanaei/3x-ui 项目
- 由 **0verf1ow** 维护和增强
- 添加了主从节点同步功能
- 优化了用户体验和界面提示

## 完整文档

详细文档请访问 [项目 Wiki](https://github.com/PeterHgg/3x-ui/wiki)

## 特别感谢

- [MHSanaei](https://github.com/MHSanaei/) - 原始 3x-ui 项目作者
- [alireza0](https://github.com/alireza0/) - 原始 X-UI 项目贡献者

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (协议: **GPL-3.0**): _增强的 v2ray/xray 路由规则，内置伊朗域名，专注于安全和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (协议: **GPL-3.0**): _基于俄罗斯被封锁域名和地址数据的自动更新 V2Ray 路由规则。_

## 支持项目

**如果这个项目对你有帮助，请给它一个**:star2:

## Star 历史

[![Stargazers over time](https://starchart.cc/PeterHgg/3x-ui.svg?variant=adaptive)](https://starchart.cc/PeterHgg/3x-ui)
