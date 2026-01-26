[English](/README.en.md) | [中文](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

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

**3X-UI** — advanced, open-source web-based control panel designed for managing Xray-core server. It offers a user-friendly interface for configuring and monitoring various VPN and proxy protocols.

> [!IMPORTANT]
> This project is only for personal usage, please do not use it for illegal purposes, and please do not use it in a production environment.

As an enhanced fork of the original X-UI project, 3X-UI provides improved stability, broader protocol support, and additional features.

## Features

### Master-Slave Node Synchronization (v2.8.x+)
- ✅ **Full Auto-Sync**: Real-time client sync to all slave nodes with auto-recovery for data consistency
- ✅ **Unified Traffic Management**: Aggregated traffic stats from all nodes for centralized quota control
- ✅ **Data Security**: Forced read-only mode on slave nodes with real-time sync status indication

### Clash Custom Subscription (v2.8.x+)
> [!NOTE]
> This configuration is specially for Cloudflare architecture, not a general subscription converter.

- ✅ **Deep Cloudflare Integration**: Auto-generated CDN nodes, forced port 443, and smart grouping by inbound remarks
- ✅ **Automated Configuration**: Auto-sync WS paths, hidden port support, and auto-set profile info (Name/Interval/Traffic)
- ✅ **Advanced Rule System**: Built-in visual rule editor with integrated Loyalsoldier/clash-rules auto-proxy

## Quick Start

### One-Click Install

```bash
bash <(curl -Ls https://raw.githubusercontent.com/PeterHgg/3x-ui/main/install.sh)
```

### One-Click Update

```bash
bash <(curl -Ls https://raw.githubusercontent.com/PeterHgg/3x-ui/main/install.sh)
```

### Management Commands

After installation, use these commands to manage the panel:

```bash
x-ui              # Show management menu
x-ui start        # Start panel
x-ui stop         # Stop panel
x-ui restart      # Restart panel
x-ui status       # View status
x-ui enable       # Enable auto-start
x-ui disable      # Disable auto-start
x-ui log          # View logs
x-ui update       # Update panel
x-ui install      # Install panel
x-ui uninstall    # Uninstall panel
```

## Version

Current Version: **v2.8.76-0verf1ow**

- Based on MHSanaei/3x-ui project
- Maintained and enhanced by **0verf1ow**
- Added master-slave node synchronization
- Integrated Clash custom subscription generator
- Optimized user experience and interface notifications
- Enhanced subscription traffic info & load balance strategy

## Documentation

For full documentation, please visit the [project Wiki](https://github.com/PeterHgg/3x-ui/wiki).

## Special Thanks

- [MHSanaei](https://github.com/MHSanaei/) - Original 3x-ui project author
- [alireza0](https://github.com/alireza0/) - Original X-UI project contributor

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _This repository contains automatically updated V2Ray routing rules based on data on blocked domains and addresses in Russia._

## Support Project

**If this project is helpful to you, you may wish to give it a**:star2:

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/PeterHgg/3x-ui.svg?variant=adaptive)](https://starchart.cc/PeterHgg/3x-ui)
