[中文](/README.md) | [English (上游原版)](https://github.com/MHSanaei/3x-ui)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/Teminuosi/3x-ui.svg)](https://github.com/Teminuosi/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/Teminuosi/3x-ui/release.yml.svg)](https://github.com/Teminuosi/3x-ui/actions)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> 🌐 **官网 / 导航站**：https://3yuedaohang.com
> 📺 **YouTube 频道**：https://www.youtube.com/@zhanzhang3yue
> 🖥️ **机器推荐**：https://3yuedaohang.com/cn2/banwagong
> 免费节点、搭建教程、订阅工具，一站直达。

**3X-UI** 是一个基于网页的 Xray-core 控制面板，用来配置和监控各种 VPN / 代理协议。

本仓库是在 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) 基础上做的**二次开发分支（fork）**，目标是让"搭协议"这件事**点几下就能完成**：内置一键协议模板、中转、多服务器下发、批量管理、扫码导入等功能，界面做了中文化优化。

> [!IMPORTANT]
> 本项目仅供个人学习与通信使用，请勿用于任何非法用途，也不建议用于生产环境。

---

## 快速开始（一条命令全自动安装）

在你的 VPS（Debian / Ubuntu / CentOS 等）上，以 root 执行下面任一条命令，零交互装完：

```bash
# 方式一：带域名 —— 自动申请 SSL 证书并把证书配到面板，其余全部用默认值
XUI_DOMAIN=panel.example.com bash <(curl -Ls https://raw.githubusercontent.com/Teminuosi/3x-ui/main/install.sh)

# 方式二：不带域名 —— 全部用默认值、随机端口，不申请证书
XUI_AUTO=1 bash <(curl -Ls https://raw.githubusercontent.com/Teminuosi/3x-ui/main/install.sh)
```

装完后，在服务器上输入 `x-ui` 即可打开管理菜单（重启面板、查看账号、改端口、更新/卸载等）。

> - 设了 `XUI_DOMAIN` 会自动开启全自动模式。若你的 shell 下行内变量没生效，可以先下载再执行：
>   ```bash
>   curl -Ls https://raw.githubusercontent.com/Teminuosi/3x-ui/main/install.sh -o /tmp/i.sh
>   XUI_DOMAIN=panel.example.com bash /tmp/i.sh
>   ```
> - 想自己一步步选（端口、SSL 方式等），去掉环境变量直接运行同一条命令即可进入交互式安装。

### 更新 / 卸载

```bash
x-ui            # 打开管理菜单，菜单里有"更新""卸载"等选项
```

---

## 本分支相比原版做了哪些改动

下面这些是本 fork 在上游 3x-ui 之上**新增 / 改造**的功能：

### 🚀 一键协议模板（推荐协议）
- "添加入站"弹窗顶部有 **推荐协议** 开关，默认开启：直接出现模板画廊，点一下就填好一整套可用配置。
- 内置 5 个主流模板：
  - **VLESS + Reality（Vision）** — 主力首选，免证书、抗封锁强（推荐）
  - **VLESS + Reality（gRPC）** — Reality 的 gRPC 变体，免证书
  - **Trojan + TLS** — 经典 TLS 伪装，需域名+证书
  - **VMess + WS + TLS** — 可走 CDN 中转，需域名+证书
  - **Hysteria2** — 基于 QUIC，速度快，需域名+证书（注意需 sing-box / NekoBox 客户端，Xray 内核不支持）
- **一键添加全部推荐**：一次性把所有可用模板都建好。面板已配好域名证书时建全部 5 个；没配证书时只建 2 个免证书的 Reality。
- 关闭"推荐协议"开关即回到完整的手动配置（协议 / 流 / 安全 / 嗅探 / 高级）。

### 🔀 中转（落地分流）
- "添加中转"按钮：入口服务器 → 落地服务器分流。
- 落地端**粘贴分享链接即可自动识别填入**（支持 vless / vmess / trojan / shadowsocks / socks / http），也可手动填写。
- 自带**连通性测试**：建完后点测试，能看到入口到落地的延迟，确认链路通不通。
- 入口默认用免证书的 Reality，自动创建入口入站 + 落地出站 + 路由规则。

### 🖧 多服务器部署（部署到）
- 在"服务器"页注册远程服务器后，添加入站时可选 **部署到** 哪台服务器（含一键模板和"一键添加全部推荐"）。
- 离线服务器会显示但灰掉不可选。

### 📋 入站 / 客户端列表增强
- **批量删除**：入站列表支持勾选 + 批量删除，并在删除时提示一并清理"孤儿客户端"（删除后不再归属任何入站的客户端）。
- **来源标识**：入站列表给中转入口打"中转"标签；客户端列表显示来源（中转 / 入站 / 独立）并支持筛选。
- **行内二维码**：每条入站直接显示**协议二维码**（多客户端则每个客户端一个），手机用小火箭等客户端直接扫码导入，不再只能复制链接。

### 🔗 共享订阅
- 推荐模板 / 一键创建的节点共用同一个订阅 ID，**一个订阅链接即可聚合全部节点**，导入客户端一次到位。

### 🈶 中文化
- "节点"统一改称"**服务器**"等界面用词优化（仅显示文案）。

> 上游原有的 SQLite / PostgreSQL 双数据库、Docker 部署等能力均保留，见下文。

---

## 常用操作指引

**搭一个协议（最常用）**
1. 左侧「入站列表」→「添加入站」。
2. 顶部「推荐协议」保持开启 → 在模板画廊点一个（默认已选中推荐的 VLESS+Reality）。
3. 只需填：备注、总流量（可留 0 = 不限）、流量重置、到期时间。
4. 点「创建」→ 回到列表，点该行的二维码图标，手机扫码导入客户端即可。

**搭中转**
1. 「入站列表」→「添加中转」。
2. 落地框粘贴落地服务器的分享链接（自动识别），或手动填写。
3. 点「测试」确认入口到落地连通（有延迟即代表通）。
4. 点「创建」，会自动建好入口入站、落地出站与路由规则。

**一个订阅管理全部**
- 用「推荐协议 / 一键添加全部推荐」创建的节点共用一个订阅链接；把该订阅地址导入客户端，即可一次性同步所有节点。

---

## 数据库选项

3X-UI 支持两种数据库后端，安装时选择：

- **SQLite**（默认）—— 单文件 `/etc/x-ui/x-ui.db`，零配置，适合中小规模部署。
- **PostgreSQL** —— 适合客户端数量大或多服务器场景。安装脚本可帮你本地装好 PostgreSQL，或填入已有数据库的 DSN。

运行时通过环境变量选择后端（安装脚本会写入 `/etc/default/x-ui`）：

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### 把现有 SQLite 迁移到 PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# 然后在 /etc/default/x-ui 中设置 XUI_DB_TYPE 与 XUI_DB_DSN，重启：
systemctl restart x-ui
```

迁移不会动原 SQLite 文件；确认新后端无误后再手动删除。

### Docker

默认 `docker compose up -d` 仍使用 SQLite。要使用内置的 PostgreSQL 服务，取消 `docker-compose.yml` 中两行 `XUI_DB_*` 环境变量的注释，并以 profile 启动：

```bash
docker compose --profile postgres up -d
```

---

## 致谢与开源协议

- 本项目基于 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)（GPL-3.0）二次开发，遵循 **GPL-3.0** 协议开源。
- 特别感谢 [alireza0](https://github.com/alireza0/)。
- 路由规则致谢：[Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules)（GPL-3.0）、[Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat)（GPL-3.0）。
