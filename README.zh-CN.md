<h1 align="center">V2Raynix</h1>

<p align="center">
  <img src="repo_assets/v2raynix-banner.png" alt="V2Raynix Banner" width="720" />
</p>

<p align="center">
  <b>新一代 Linux 全局网络分流与透明代理管理器</b><br/>
  <i>基于 Go、Xray-core、tun2socks、Linux 内核策略路由与内置 React Web 仪表盘打造</i>
</p>

<p align="center">
  <a href="https://github.com/v2raynix/v2raynix/releases"><img src="https://img.shields.io/badge/版本-v0.9.0--beta-orange.svg?style=flat-square" alt="版本"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-%3E%3D1.23-blue.svg?style=flat-square" alt="Go 版本"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/协议-MIT-green.svg?style=flat-square" alt="开源协议"></a>
  <img src="https://img.shields.io/badge/平台-Linux%20(amd64%20%7C%20arm64)-purple.svg?style=flat-square" alt="平台">
  <img src="https://img.shields.io/badge/状态-Active%20Beta-success.svg?style=flat-square" alt="状态">
</p>

<p align="center">
  <a href="README.md"><b>English</b></a> •
  <a href="README.fa.md">🇮🇷 <b>فارسی</b></a> •
  <b>🇨🇳 简体中文</b> •
  <a href="README.ru.md">🇷🇺 <b>Русский</b></a>
</p>

---

## 📑 目录

- [⚡ 一键安装脚本](#-一键安装脚本)
- [💡 为什么选择 V2Raynix？](#-为什么选择-v2raynix)
- [🚀 核心功能特性](#-核心功能特性)
- [🏛️ 系统架构设计](#-系统架构设计)
- [🖥️ 终端交互式菜单 (TUI)](#-终端交互式菜单-tui)
- [🌐 现代 Web 管理面板](#-现代-web-管理面板)
- [⚙️ CLI 命令行参数参考](#-cli-命令行参数参考)
- [🛠️ 从源码编译构建 (可选)](#-从源码编译构建-可选)
- [💖 支持与赞助 (捐赠)](#-支持与赞助-捐赠)
- [🔒 安全说明与免责声明](#-安全说明与免责声明)
- [📄 开源许可证](#-开源许可证)

---

## ⚡ 一键安装脚本

在任何 Linux 服务器（**Ubuntu**、**Debian**、**CentOS**、**Fedora** 或 **Arch Linux**）上仅需**执行单条命令**即可快速部署并自动启动 V2Raynix —— **无需预装 Go、Node.js，也无需任何手动编译步骤**：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/v2raynix/v2raynix/master/scripts/install.sh)
```

### 🪄 安装脚本自动执行的操作：
1. **自动检测 CPU 架构：** 智能识别 `x86_64` (`amd64`) 或 `aarch64` (`arm64`)。
2. **安装系统依赖：** 自动检测并安装 `curl`、`unzip`、`iptables`、`iproute2` 和 `ca-certificates`。
3. **部署官方核心引擎：** 自动下载配置最新的官方 **Xray-core** 与 **tun2socks**，并部署最新的 **GeoIP** 和 **GeoSite** 路由资源规则库。
4. **安装独立可执行程序：** 下载预先打包好前端的 `v2raynix` 独立单二进制文件至 `/usr/local/bin/v2raynix`。
5. **自动放行防火墙端口：** 在 `ufw` 或 `firewalld` 中放行 Web 控制台端口 `2080`。
6. **创建并启动系统服务：** 生成 `v2raynix.service` 并立即通过 systemd 后台常驻启动。

### 🌐 访问 Web 控制面板：
安装完成后，在浏览器中打开：
```text
http://<您的服务器IP>:2080
```
- **默认用户名：** `admin`
- **默认密码：** `admin` *(首次登录或通过终端设置菜单时会提示您修改密码)*

---

## 💡 为什么选择 V2Raynix？

在无图形界面的无头 (Headless) Linux 服务器上管理代理客户端历来繁琐、脆弱且极具风险。传统的命令行工具通常需要手动编写晦涩复杂的 JSON 配置、缺乏统一的虚拟网卡 (TUN) 设备管理；更致命的是，一旦修改了系统默认路由表，往往会导致 **SSH 会话瞬间中断并发生网络黑洞**，将管理员彻底锁在云服务器之外（失联）。

**V2Raynix** 通过创新的工程化方案彻底解决了这些痛点：
1. **防失联内核策略路由 (Zero-Lockout Routing)：** 自动侦测当前 SSH 端口、管理员来路 IP 及默认网关物理出口，确保所有远程运维连接永远直连，绝不因代理切换而断连。
2. **120秒网络安全倒计时 (Safe Mode)：** 任何路由规则变更均会自动触发 120 秒回滚保护机制；若未能在此期间获得连通确认，系统将自动复原原始网络表，杜绝断网失联风险。
3. **零依赖单二进制发布：** 现代 React 响应式管理前端直接被编译嵌入在 Go 可执行文件内 (`go:embed`)。服务器无需安装 Node.js、Nginx 或外部数据库。
4. **终端交互式控制台 (TUI)：** 提供带有 ANSI 配色与边框的终端设置菜单 (`v2raynix setup`)，支持 Linux `sudo` 式静默密码掩码输入及双重密码校验，无需浏览器即可秒级重置凭据或配置端口。

---

## 🚀 核心功能特性

- **🌐 全局透明网络代理 (`tun2socks`)：** 通过虚拟网络接口 `tun0` 透明代理 Linux 服务器所有出站 TCP/UDP 流量，无需各软件单独配置代理环境变量。
- **🛡️ 双层防断联防护机制：** 内核策略路由规则确保 SSH 连接（端口 22 或自定义端口）、本地 Web 面板及物理网关流量绕行 `tun0`。
- **⏱️ 自动化安全倒计时保护：** 120 秒网络安全模式避免配置错误导致网络黑洞。
- **⚡ 多协议全量支持：**
  - **VLESS:** 完美支持 `xhttp`、`reality`、`xtls-rprx-vision`、`ws`、`grpc` 和 `tcp`。
  - **VMess:** 支持完整 AEAD 鉴权以及 WebSocket、TCP 伪装头部。
  - **Trojan:** 支持标准 TLS 与 WebSocket 连接。
  - **Shadowsocks:** 兼容 2022 最新 AEAD 算法及经典加密方式。
- **🧠 智能分流规则与地理数据纠错规范化：** 支持直连 (Direct)、代理 (Proxy)、拦截 (Block) 的域名与 IP 规则。自动规范化易错别名（例如将 `geosite:ir` 自动转为 `geosite:category-ir`），彻底杜绝因规则库语法不兼容引发的核心引擎崩溃。
- **📊 实时多维度节点延迟诊断：** 同时支持底层 TCP 连接探测与真实上层 HTTP 握手时延测试。
- **🖥️ 静默输入交互终端 TUI：** 终端设置向导 `v2raynix setup` 采用无回显静默密码输入，并强制进行重复确认，防止误输入。
- **🔄 面板内一键核心热更新：** 可在 Web 界面中直接在线检查并热升级官方 `Xray-core` 和 `tun2socks` 核心，具备校验和哈希验证与原子回滚机制。

---

## 🏛️ 系统架构设计

```text
[ 系统进程及所有出站网络流量 ]
              │
              ▼
   ┌─────────────────────────────────────┐
   │         Linux 内核策略路由          │
   └──────────────────┬──────────────────┘
                      │
       ┌──────────────┴──────────────┐
       │ (SSH 管理 / 面板端口 / 目标IP)│ (所有默认系统流量)
       ▼                             ▼
┌──────────────┐              ┌──────────────┐
│  物理以太网卡 │              │  tun0 虚拟   │
│   Default GW │              │   网络接口   │
└──────────────┘              └──────┬───────┘
                                     │
                                     ▼
                              ┌──────────────┐
                              │  tun2socks   │
                              └──────┬───────┘
                                     │ (SOCKS5 127.0.0.1:10808)
                                     ▼
                              ┌──────────────┐
                              │  Xray Core   │
                              └──────┬───────┘
                                     │ (VLESS / VMess / Trojan / SS)
                                     ▼
                                [ 远端节点 ]
```

---

## 🖥️ 终端交互式菜单 (TUI)

当您无法打开浏览器，或者需要通过 SSH 快速重置管理员密码、更改面板监听端口或检查服务健康状态时，只需执行：

```bash
sudo v2raynix setup
```

```text
┌────────────────────────────────────────────────────────────┐
│                    V2RAYNIX SERVER SETUP                   │
├────────────────────────────────────────────────────────────┤
│  Service Status : ● Active (Running)                       │
│  Web Management : http://127.0.0.1:2080                    │
├────────────────────────────────────────────────────────────┤
│  [1] Change / Reset Admin Credentials                      │
│  [2] Change Web Panel Listening Port                       │
│  [3] Manage V2Raynix Service (Start / Stop / Restart)      │
│  [4] View Server Status & Diagnostics                      │
│  [0] Exit Setup                                            │
└────────────────────────────────────────────────────────────┘
```

> **安全提示：** 选择选项 `[1]` 修改密码时，键盘输入的内容将完全不回显任何字符或星号（与 Linux `sudo` 密码输入行为完全一致），并会要求再次输入确认，确保凭据安全与准确。

---

## 🌐 现代 Web 管理面板

内嵌的 Web 交互界面具备响应式深色半透明毛玻璃美学设计：

- **系统仪表盘：** 实时监控全机网络上下行流量速率、连接运行时长、虚拟 TUN 网卡状态与安全模式倒计时。
- **节点配置管理：** 支持一键导入标准分享链接（`vless://`、`vmess://`、`trojan://`、`ss://`）或原始 JSON，支持批量 TCP 延迟与真实 HTTP 握手延迟测试。
- **智能策略路由：** 可基于域名、IP 或 CIDR 网段自由创建直连、代理或拦截策略，内置常用规则预设（如国内分流直连、广告拦截等）。
- **实时日志监控：** 实时流式输出 Linux 内核路由事件、后台守护进程状态转换以及 Xray 核心运行日志。
- **系统设置与内核更新：** 支持修改监听端口、调整安全模式倒计时时长、重置管理员密码，并提供一键免重启热更新核心组件。

---

## ⚙️ CLI 命令行参数参考

```text
Usage: v2raynix [flags]
       v2raynix setup [subcommand flags]

Flags:
  -port int
        Web 界面及 REST API 监听端口 (默认: 2080)
  -data-dir string
        数据及配置文件存储目录 (默认: /etc/v2raynix 或 ./data)
  -init-password string
        直接初始化或重设管理员密码
  -mock
        在模拟模式下运行，不修改内核网络接口与 iptables
  -version
        输出 V2Raynix 版本信息并退出
```

---

## 🛠️ 从源码编译构建 (可选)

<details>
<summary><b>点击展开查看开发者手动编译指南</b></summary>
<br/>

如果您希望直接从源代码进行编译，而非使用自动一键安装脚本：

### 前置依赖要求：
- **Go:** 1.23 或更高版本
- **Node.js & npm:** 18+（用于编译内嵌的前端 React SPA）
- **Bash / Make:** 基础 Linux 构建环境

### 编译构建流程：

```bash
# 1. 克隆代码仓库
git clone https://github.com/v2raynix/v2raynix.git
cd v2raynix

# 2. 编译打包 React 前端应用
cd web
npm install
npm run build
cd ..

# 3. 编译打包单一独立 Go 二进制执行程序
go build -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix

# 4. 在模拟模式下本地运行测试
./bin/v2raynix -port 2080 -mock
```
</details>

---

## 💖 支持与赞助 (捐赠)

V2Raynix 是一个完全独立、免费开源的项目，致力于推动互联网自由与开放透明的网络通信。如果您觉得本项目为您的工作与网络连接提供了帮助，欢迎通过加密货币赞助支持作者的持续维护与迭代开发！

### 📋 赞助收款钱包地址

| 网络 / 加密货币 | 钱包地址 |
| :--- | :--- |
| **BNB Smart Chain (BEP20)** | `0x726524eF2Bf606f12829C7724a37196E5fE00F44` |
| **Tron (TRC20)** | `TYkdrBjmJEMxXbxS6pwCujHvB18AS9WZ57` |
| **Bitcoin (BTC)** | `bc1qjyjat944wz466l3pz27953gfl69ey2wf66r2x4` |
| **Solana (SOL)** | `GevJAdW3x8Y8sqgDmYQf6VXGsoNny3hh8gFJthknC61W` |
| **Ethereum (ERC20)** | `0x726524eF2Bf606f12829C7724a37196E5fE00F44` |

<br/>

### 📱 扫码赞助 (QR 码)

<div align="center">

| **BNB (BEP20)** | **Tron (TRC20)** | **Bitcoin (BTC)** | **Solana (SOL)** | **Ethereum (ERC20)** |
| :---: | :---: | :---: | :---: | :---: |
| <a href="repo_assets/qr-bnb.png"><img src="repo_assets/qr-bnb.png" width="95" alt="BNB" /></a> | <a href="repo_assets/qr-trx.png"><img src="repo_assets/qr-trx.png" width="95" alt="TRX" /></a> | <a href="repo_assets/qr-btc.png"><img src="repo_assets/qr-btc.png" width="95" alt="BTC" /></a> | <a href="repo_assets/qr-sol.png"><img src="repo_assets/qr-sol.png" width="95" alt="SOL" /></a> | <a href="repo_assets/qr-eth.png"><img src="repo_assets/qr-eth.png" width="95" alt="ETH" /></a> |
| `0x7265...0F44` | `TYkd...WZ57` | `bc1q...r2x4` | `GevJ...C61W` | `0x7265...0F44` |

</div>

---

## 🔒 安全说明与免责声明

- **Root 权限：** 全局网络流量分流与策略路由（`tun0`、`iptables`、`ip route`）需要 Linux 系统的网络管理特权（`CAP_NET_ADMIN`、`CAP_NET_BIND_SERVICE`）。后台守护进程在非必要时会自动剥离多余权限。
- **免责声明：** 本软件按“现状”提供，不包含任何形式的明示或暗示保证。使用者需自行遵守所在国家与地区的法律法规与电信监管政策。

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源协议授权发布。
欢迎提交 Issue、功能建议与 Pull Request！
