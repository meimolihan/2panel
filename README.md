# 2Panel · 计划任务

一个精简的开源计划任务（定时任务）管理工具，参考 [1Panel](https://github.com/1Panel-dev/1Panel) 的计划任务功能重构，**只保留计划任务这一个能力**。

- 单二进制部署：Go 后端 + 内嵌 Web 管理页面，无需 Node/外部依赖
- SQLite 持久化（纯 Go 驱动，无需 CGO / GCC）
- 支持 **shell 脚本** 与 **curl 请求** 两类任务
- 完整 CRUD：创建 / 编辑 / 删除 / 启停 / 手动执行 / 停止执行中的任务
- 执行记录：历史记录、耗时、状态、实时日志查看（支持 ANSI 彩色日志）
- cron 表达式（5 段）与 `@every` / `@daily` 等快捷语法，支持多周期 `&&` 拼接
- **脚本库**：集中管理可复用脚本，任务可选择引用脚本库（改脚本即时生效，无需改任务）
- **导入 / 导出**：任务一键导出为 JSON 文件，导入时自动跳过重名、统计失败项
- **崩溃自愈**：服务重启时自动清理执行中残留状态，恢复全部启用的任务调度（执行记录会标记为「interrupted by service restart」）

## 快速开始

```bash
# 构建
go build -o 2panel .

# 运行（默认端口 8080，数据目录 ./data）
./2panel

# 指定端口 / 数据目录
./2panel -port 18080 -data /var/lib/2panel
```

浏览器打开 `http://<服务器IP>:8080` 即可使用。

## 在服务器上部署（后台运行 / systemd 常驻）

### 1. 构建并后台运行

```bash
cd /vol1/1000/compose/opencode/workspace/test/2Panel   # 换成你的项目路径
go build -o 2panel . && \
nohup ./2panel >/dev/null 2>&1 &
```

> 想看日志可改成 `nohup ./2panel > data/2panel.log 2>&1 &`，用 `tail -f data/2panel.log` 查看。

### 2. 验证服务

```bash
# 服务是否在运行
ps aux | grep "[2]panel"

# Web 管理界面（应返回 200）
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/

# 搜索接口（POST，需先登录获取 token，应返回 200）
curl -sS -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"page":1,"pageSize":10}' \
  http://127.0.0.1:8080/api/cronjobs/search
```

> 注意：所有 `/api/cronjobs/*` 接口都是 POST 方法，用 GET 访问会落到静态文件服务返回 404，属正常现象。

### 2.1 登录与初始密码

- 浏览器访问 `http://<服务器IP>:8080/`，首次进入会看到登录页。
- 服务首次启动时会自动初始化管理员账号并生成随机**默认密码**，可通过以下任一方式查看：
  ```bash
  # 1) 服务启动日志
  journalctl -u 2panel -n 20 | grep "default password"
  #    或（nohup 方式）
  grep "default password" data/2panel.log

  # 2) 状态接口（未改密码前返回）
  curl -sS -X POST -H "Content-Type: application/json" -d '{}' \
    http://127.0.0.1:8080/api/auth/status
  ```
- 默认用户名 `admin`，登录后请立即在右上角「修改密码」中更换初始密码；修改成功后默认密码即失效。
- 登录凭证为内存中的 token（24 小时有效），服务重启后需重新登录。

停止后台进程：`pkill -f '^./2panel'`

### 3. systemd 开机自启（推荐）

> 使用 `install.sh` 安装时**自动注册 systemd 服务**（开机自启 + 崩溃自动重启 + 日志统一由 journalctl 管理），无需手动配置。
> 只有系统没有 systemd（如容器环境）时才回退为后台运行模式。

手动部署（例如本地 `go build` 生成二进制）时，可按下述步骤注册：

```bash
# 先停掉 nohup 后台进程，释放端口
systemctl stop 2panel 2>/dev/null; pkill -f '^./2panel'; sleep 1
```

```bash
# 创建服务文件（粘贴后按 Ctrl+D 结束）
tee /etc/systemd/system/2panel.service <<'EOF'
[Unit]
Description=2Panel - Scheduled Task Manager
After=network.target

[Service]
Type=simple
WorkingDirectory=/vol1/1000/compose/opencode/workspace/test/2Panel
ExecStart=/vol1/1000/compose/opencode/workspace/test/2Panel/2panel
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
```

```bash
# 加载并立即启动（enable 开启开机自启）
systemctl daemon-reload
systemctl enable --now 2panel

# 验证
systemctl status 2panel --no-pager     # 应显示 active (running)
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/
```

### 4. 日常管理命令

| 操作 | 命令 |
| --- | --- |
| 查看状态 | `systemctl status 2panel` |
| 停止 / 启动 / 重启 | `systemctl stop 2panel` / `systemctl start 2panel` / `systemctl restart 2panel` |
| 查看实时日志 | `journalctl -u 2panel -f` |
| 查看最近 30 行日志 | `journalctl -u 2panel -n 30 --no-pager` |
| 关闭开机自启 | `systemctl disable 2panel` |

### 5. 更新（改完代码后重新部署）

```bash
cd /vol1/1000/compose/opencode/workspace/test/2Panel
go build -o 2panel . && systemctl restart 2panel
```

## 部署到 GitHub 并远程安装

### 1. 推送代码到 GitHub

```bash
git init
git add .
git commit -m "feat: 2Panel scheduled task manager"
git remote add origin https://github.com/<你的用户名>/2Panel.git
git push -u origin main
```

### 2. 修改 install.sh 中的占位符

打开 `install.sh`，把 `GITHUB_OWNER` 改成你的 GitHub 用户名，然后推送：

```bash
# install.sh 顶部
GITHUB_OWNER="YOUR_GITHUB_USERNAME"   # 改成你的用户名
```

### 3. 构建 release 附件并发布

```bash
# 交叉编译 linux amd64 / arm64 两个二进制（无需目标机器）
./scripts/build-release.sh                  # 默认 v0.1.0，可用 VERSION=v1.0.0 覆盖

# 发布 GitHub Release（需安装 gh CLI）
VERSION=v0.1.0
gh release create "$VERSION" dist/2panel_linux_amd64 dist/2panel_linux_arm64 \
  --title "$VERSION" --notes "2Panel scheduled task manager"
```

> Release 附件名称必须为 `2panel_linux_amd64` 与 `2panel_linux_arm64`，install.sh 依赖该命名下载。

### 4. 在其他机器上一键安装

在任意 Linux 服务器（root 或 sudo）上执行：

```bash
bash -c "$(curl -sSL https://raw.githubusercontent.com/<你的用户名>/2Panel/main/install.sh)"
```

安装脚本会自动完成：

1. 检测系统架构（amd64 / arm64），从 GitHub Release 下载对应二进制
2. **交互式提示输入监听端口**（默认 8080，校验 1-65535）
3. 提示输入数据目录（默认 `/var/lib/2panel`）
4. **自动注册为 systemd 服务**（开机自启 + 崩溃自动重启 + journald 日志）；系统无 systemd（如容器）时自动回退为后台运行并给出提示
5. 打印访问地址 `http://<服务器IP>:<端口>`

```bash
$ bash -c "$(curl -sSL https://raw.githubusercontent.com/<用户名>/2Panel/main/install.sh)"
============================================================
 Installing 2Panel
   OS   : Linux x86_64
   Arch : amd64
============================================================
Enter listen port [default: 8080]: 18080        # ← 输入端口号
Enter data directory [default: /var/lib/2panel]: # 回车用默认
>>> 2panel service started.                      # ← 自动注册并启动 systemd 服务
...
  Web UI   : http://192.168.1.10:18080
```

### 卸载

```bash
# 方式一：使用卸载脚本（本地克隆或下载后执行）
bash uninstall.sh
# 或远程执行
bash -c "$(curl -sSL https://raw.githubusercontent.com/<用户名>/2Panel/main/uninstall.sh)"
```

脚本会依次：停止并移除 systemd 服务 → 结束后台运行进程 → 删除二进制 → **询问是否删除数据目录**（默认保留，数据目录含数据库/脚本/日志，请确认后再删）。

```bash
# 方式二：手动卸载
systemctl stop 2panel && systemctl disable 2panel        # systemd 模式
rm -f /etc/systemd/system/2panel.service && systemctl daemon-reload
pkill -f "^/usr/local/bin/2panel"                          # 后台模式
rm -f /usr/local/bin/2panel                               # 删除二进制
rm -rf /var/lib/2panel                                    # 删除数据（数据库/脚本/日志）
```

### 升级

```bash
# 直接替换二进制后重启
bash -c "$(curl -sSL https://raw.githubusercontent.com/<用户名>/2Panel/main/install.sh)"   # 重跑即覆盖二进制
systemctl restart 2panel   # systemd 模式
```

### 命令行参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-port` | `8080` | HTTP 监听端口 |
| `-data` | 二进制同目录 `data/` | 数据目录（数据库、脚本、日志） |
| `-debug` | `false` | 是否输出调试日志 |
| `-version` | - | 输出版本号 |

## 目录结构

```
2Panel/
├── main.go                          # 入口：参数解析、数据库、调度器、启动 HTTP
├── install.sh                       # 远程一键安装脚本（交互式输入端口等）
├── uninstall.sh                     # 卸载脚本（停服务/删进程/删二进制/可选删数据）
├── scripts/
│   └── build-release.sh             # 交叉编译 GitHub Release 附件
├── internal/
│   ├── database/database.go         # SQLite 初始化与自动建表
│   ├── model/cronjob.go             # Cronjob / JobRecord 数据模型
│   ├── model/script_library.go      # 脚本库数据模型
│   ├── repo/cronjob.go              # 数据访问层（分页、条件查询、记录管理）
│   ├── repo/script_library.go       # 脚本库数据访问层
│   ├── dto/cronjob.go               # 请求 / 响应结构体
│   ├── dto/script_library.go        # 脚本库请求 / 响应结构体
│   ├── scheduler/
│   │   ├── scheduler.go             # robfig/cron 调度、shell/curl 执行、超时与停止
│   │   └── log_writer.go            # 执行日志落盘 + 内存缓冲
│   ├── service/cronjob.go           # 业务逻辑（注册调度、执行、记录、导入导出）
│   ├── service/script_library.go    # 脚本库业务逻辑
│   ├── handler/cronjob.go           # HTTP 处理函数
│   ├── handler/script_library.go    # 脚本库 HTTP 处理函数
│   └── server/server.go             # 路由 + 内嵌 Web 静态资源
└── internal/server/web/index.html   # 单页管理界面（无外部 CDN）
```

## API

- 认证接口以 `/api/auth` 为前缀，其余以 `/api/cronjobs` 为前缀，返回 `{"code":0,"msg":"success","data":...}`。
- 除 `/api/auth/*` 外，所有接口都需要请求头 `Authorization: Bearer <token>`（登录返回的 token）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/status` | 认证状态（含是否显示初始密码） |
| POST | `/api/auth/login` | 登录，返回 `{token, name}` |
| POST | `/api/auth/logout` | 退出登录 |
| POST | `/api/auth/change-password` | 修改密码 |
| POST | `/api/cronjobs` | 创建任务 |
| POST | `/api/cronjobs/search` | 分页查询任务 |
| POST | `/api/cronjobs/load/info` | 加载单个任务 |
| POST | `/api/cronjobs/update` | 更新任务 |
| POST | `/api/cronjobs/del` | 批量删除 |
| POST | `/api/cronjobs/status` | 启用 / 停用 |
| POST | `/api/cronjobs/handle` | 手动执行一次 |
| POST | `/api/cronjobs/stop` | 停止执行中的任务 |
| POST | `/api/cronjobs/next` | 计算未来 5 次执行时间 |
| POST | `/api/cronjobs/search/records` | 分页查询执行记录 |
| POST | `/api/cronjobs/records/log` | 读取执行日志 |
| POST | `/api/cronjobs/export` | 导出全部任务（JSON） |
| POST | `/api/cronjobs/import` | 导入任务，返回 `{created,skipped,failed}` |
| POST | `/api/cronjobs/script/options` | 脚本库名称列表（任务编辑器下拉用） |
| POST | `/api/scripts/search` | 分页查询脚本库 |
| POST | `/api/scripts/load/info` | 加载单个脚本 |
| POST | `/api/scripts/create` | 新增脚本 |
| POST | `/api/scripts/update` | 更新脚本 |
| POST | `/api/scripts/del` | 删除脚本 |

### 登录示例

```json
POST /api/auth/login
{ "name": "admin", "password": "<初始密码>" }
// 返回：{"code":0,"data":{"token":"<token>","name":"admin"}}

POST /api/auth/change-password          // 请求头需带 Authorization
{ "oldPassword": "<旧密码>", "newPassword": "<新密码，至少 8 位>" }
```

### 创建任务示例

```json
POST /api/cronjobs
{
  "name": "daily-clean",
  "type": "shell",                       // shell | curl
  "spec": "0 2 * * *",                   // cron 表达式或 @every 1h
  "executor": "bash",                    // shell 类型：bash / sh / python3 ...
  "script": "rm -rf /tmp/cache/*",       // shell 类型：脚本内容
  "scriptName": "lib-backup",            // 可选：引用脚本库中的脚本（执行时取最新内容）
  "user": "root",                        // 可选：以指定用户执行
  "url": "https://example.com/ping",     // curl 类型：URL，多个用逗号分隔
  "timeout": 30,                         // 超时（秒），0 表示不限
  "retryTimes": 0,                       // 失败重试等待间隔（秒）
  "retainCopies": 7                      // 执行记录及日志保留份数，0 表示全部保留
}
```

### 任务类型

- **shell**：将脚本内容写入 `data/task/<名称>/` 后，用指定执行器（默认 `bash`）运行，支持 `python` 等解释器；可指定执行用户。若设置了 `scriptName` 引用脚本库，执行时解析该脚本的最新内容。
- **curl**：依次 `curl -sSL` 访问一个或多个 URL，任意一个失败即整次执行失败。

### 脚本库

- 在「脚本库」页签集中管理可复用脚本（名称唯一，含描述与内容）。
- 创建 / 编辑任务时，脚本来源可选「手动输入」或「脚本库」；引用脚本库时任务仅保存脚本名，执行时动态解析最新内容，脚本库中修改后无需重新保存任务。
- 若引用的脚本被删除，任务执行将报错 `the script content is empty`。
- 导入 / 导出仅涉及任务，不包含脚本库内容，请先导出脚本库以便迁移。

### 调度规则

- 支持标准 5 段 cron 表达式：`分 时 日 月 周`（如 `0 2 * * *`）
- 支持快捷语法：`@every 5m`、`@every 1h`、`@daily`、`@weekly` 等
- 服务重启后自动恢复所有已启用任务（`RestoreCronjobs`）
- 任务执行中会跳过/顺延下一次触发（`DelayIfStillRunning`），避免并发堆积

## 数据与日志

- 数据库：`<data>/2panel.db`
- 任务脚本：`<data>/task/<任务名>/<任务名>.sh`
- 执行日志：`<data>/log/<taskID>.log`

## 与 1Panel 的对应关系

| 1Panel | 2Panel |
| --- | --- |
| `agent/app/model/cronjob.go` | `internal/model/cronjob.go` |
| `agent/app/repo/cronjob.go` | `internal/repo/cronjob.go` |
| `agent/app/service/cronjob.go` | `internal/service/cronjob.go` |
| `agent/app/api/v2/cronjob.go` | `internal/handler/cronjob.go` |
| `agent/router/ro_cronjob.go` | `internal/server/server.go` |
| `agent/init/cron/` | `internal/scheduler/` |
| `frontend/src/views/cronjob/*` | `internal/server/web/index.html` |

2Panel 砍掉了 1Panel 中与计划任务无关的模块（网站、应用、数据库、备份、告警、AI Agent 等），仅保留 shell / curl 两类任务及执行记录，其余保持相同的分层结构与 API 风格。
