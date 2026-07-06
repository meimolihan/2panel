<p align="center"><img src="https://resource.1panel.pro/img/1panel-logo.png" alt="2Panel" width="300" /></p>

<h3 align="center">轻量级 Linux 服务器管理面板</h3>

<p align="center">
  <strong>2Panel</strong> 是一款基于 1Panel 精简优化的 Linux 服务器管理面板
</p>

---

## 功能概览

2Panel 保留了 1Panel 的核心功能，并移除了应用商店、网站管理、数据库管理等模块，专注于以下核心功能：

| 功能 | 说明 |
|------|------|
| 📊 **概览** | 服务器运行状态监控，资源使用概览 |
| 🐳 **容器** | Docker 容器、镜像、网络、存储卷、Compose 编排管理 |
| ⚙️ **系统** | 系统监控、防火墙、SSH 管理、文件管理、磁盘管理、进程管理 |
| 💻 **终端** | 浏览器内建 Web 终端，支持多主机管理 |
| 📅 **计划任务** | 定时备份、日志清理、脚本执行等计划任务管理 |
| 🧰 **工具箱** | 系统工具箱：Fail2ban、FTP、ClamAV 杀毒、系统清理、Supervisor |
| 📋 **日志审计** | 操作日志、登录日志、系统日志、任务日志 |
| ⚡ **面板设置** | 面板配置、备份账号、快照管理、**全量备份与恢复** |

## 全量备份与恢复

2Panel 新增全量备份与全量恢复功能，支持：

- **全量备份**：一键备份面板所有数据，包括数据库、配置、日志等
- **全量恢复**：从备份文件完整恢复面板数据
- 备份文件存储为 `.tar.gz` 格式，支持本地存储

## 快速开始

### 在线安装

```bash
curl -sSL https://resource.2panel.pro/v2/install/quick_start.sh -o quick_start.sh && bash quick_start.sh
```

### 环境要求

- 操作系统：Linux (CentOS 7+/Ubuntu 20.04+/Debian 11+)
- 架构：x86_64 / arm64
- 需安装 Docker

## 构建

```bash
# 后端构建
cd agent && go build -o 2panel-agent ./cmd/server/

# 前端构建
cd frontend && npm install && npm run build
```

## 开源许可

基于 GPL v3 协议开源。
