# lsm — Lsm iS Manager

跨平台的命令行后台服务管理工具。用一个 YAML 文件登记若干服务，然后用 `lsm` 启动、停止、重启、查看它们。

- **单文件二进制，零运行时依赖**：下载即用。
- **管理程序不常驻**：`lsm start` 执行完自身退出，被管理的服务以脱离终端的后台进程继续运行。
- **命令字符串原样透传**：命令交给系统 shell 解析，`lsm` 不拆分、不转义、不重写。
- **输出干净**：被管理服务的 stdin / stdout / stderr 全部丢弃，不会污染 `lsm` 自己的输出。

支持 Linux、macOS、Windows（amd64 / arm64）。

---

## 安装

需要 **Go 1.24 或更高版本**（原因见下文「设计说明」：Go 1.22 生成的 macOS 二进制无法被新版 `dyld` 加载）。

```sh
# 用 Go 安装
go install github.com/LiZeC123/lsm@latest

# 或者从源码构建
git clone https://github.com/LiZeC123/lsm.git
cd lsm
go build -o lsm .
```

---

## 快速开始

```sh
# 1. 交互式登记一个服务
$ lsm add
Service name: web
Command: node /home/user/web/server.js --port 3000
Working directory: /home/user/web
Service 'web' added.

# 2. 启动它
$ lsm start web
service 'web' started (pid 12345)

# 3. 看看状态
$ lsm ps
NAME  STATUS   PID    UPTIME
web   running  12345  2h13m

# 4. 停止并删除
$ lsm stop web
service 'web' stopped
$ lsm del web
Delete service 'web'? [y/N]: y
Service 'web' deleted.
```

---

## 命令

| 命令                 | 说明                                       |
| -------------------- | ------------------------------------------ |
| `lsm start <name>`   | 启动指定服务                               |
| `lsm stop <name>`    | 停止指定服务（停止未运行的服务也算成功）   |
| `lsm restart <name>` | 重启指定服务；未运行则直接启动             |
| `lsm ps`             | 列出所有服务的运行状态                     |
| `lsm list`           | 列出所有已配置的服务                       |
| `lsm add`            | 交互式添加服务                             |
| `lsm del <name>`     | 删除服务配置（`--force` 会先停止再删除）   |

全局参数：`--config <path>`。

```
$ lsm ps
NAME    STATUS   PID    UPTIME
web     running  12345  2h13m
api     stopped  -      -
worker  running  12401  8m

$ lsm list
NAME  COMMAND                                    WORK_DIR
api   ./bin/api -config ./conf/api.yaml          /opt/services/api
web   node /home/user/web/server.js --port 3000  /home/user/web
```

---

## 配置文件

配置目录由 Go 标准库 `os.UserConfigDir()` 决定：

| 平台    | 路径                                            |
| ------- | ----------------------------------------------- |
| Linux   | `~/.config/lsm/config.yaml`                     |
| macOS   | `~/Library/Application Support/lsm/config.yaml` |
| Windows | `%AppData%\lsm\config.yaml`                     |

查找优先级从高到低：`--config <path>` → 环境变量 `LSM_CONFIG` → 平台默认路径。

### 格式

```yaml
services:
  web:
    command: 'node /home/user/web/server.js --port 3000'  # 必填，完整 shell 命令
    work_dir: '/home/user/web'                            # 必填，绝对路径
```

字段规则：

| 字段       | 规则                                                            |
| ---------- | --------------------------------------------------------------- |
| 服务名     | 唯一，大小写敏感，匹配 `^[A-Za-z0-9_-]+$`，长度 1–64            |
| `command`  | 非空，原样存储并交给系统 shell                                  |
| `work_dir` | 非空，绝对路径，写回时统一转换为正斜杠；目录是否存在由命令负责  |

配置文件可以直接手改。`lsm` 写回时使用 `gopkg.in/yaml.v3` 序列化，**不保留注释**，并采用原子写入（临时文件 + `fsync` + `rename`），进程崩溃不会导致配置被截断。

路径答案可以写相对路径，`lsm add` 会把它解析成绝对路径再存储。

### 含特殊字符的示例

```yaml
services:
  web:
    command: 'node ./server.js --bind "127.0.0.1:8080"'
    work_dir: '/home/user/web'
  pipeline:
    command: 'cat input.txt | grep -v "^#" | sort > output.txt'
    work_dir: '/data/pipeline'
  win-path:
    command: 'C:\Python311\python.exe app.py'
    work_dir: 'C:/work/app'
```

`command` 里的引号、反斜杠、管道、重定向和 `$变量` 全部由系统 shell（Unix 上是 `/bin/sh`，Windows 上是 `cmd.exe`）解析，`lsm` 不介入。

---

## 运行时目录与 PID 文件

PID 文件位于配置文件同级的 `run/` 子目录（权限 `0700`，文件权限 `0600`）：

```
<configDir>/lsm/
├── config.yaml
└── run/
    ├── web.pid
    └── api.pid
```

```json
{
  "pid": 12345,
  "started_at": 1712345678,
  "work_dir": "/home/user/web",
  "command": "node /home/user/web/server.js --port 3000"
}
```

`started_at` 用于计算 `ps` 的 UPTIME，同时也是判断 PID 是否被复用的依据。

---

## 退出码

| 码  | 含义                       |
| --- | -------------------------- |
| 0   | 成功                       |
| 1   | 通用错误（未分类）         |
| 2   | 命令行用法错误             |
| 3   | 服务不存在                 |
| 4   | 服务已在运行               |
| 5   | 服务未运行（保留）         |
| 6   | 配置读取 / 解析 / 校验失败 |
| 7   | 配置写入失败               |
| 8   | 进程操作失败               |

错误输出到 stderr，格式为单行 `lsm: <message>`，不打印堆栈。设置 `LSM_DEBUG=1` 会额外输出底层错误链。

---

## 设计说明与已知限制

- **启动后存活确认窗口为 150ms。** 命令经由 shell 启动，所以「可执行文件不存在」不会让启动失败，只会让 shell 立刻退出。`lsm` 在 `Start()` 后等待 150ms 再检查进程是否存活，若已退出则视为启动失败（退出码 8）且不写 PID 文件。因此**执行完就退出的「一次性任务」不适合用 `lsm` 管理**。
- **停止方式是强力的。** Unix 上向进程组发送 `SIGTERM`，最多等待 10 秒后升级为 `SIGKILL`；Windows 上直接 `taskkill /PID <pid> /T /F`（detached 控制台进程没有可接收 `WM_CLOSE` 的窗口）。
- **PID 复用保护。** 记录的 PID 可能被系统回收给无关进程。Unix 上 `lsm` 只对「PGID 等于 PID」的进程组发信号（`Setsid` 保证自己启动的进程一定满足），Windows 上会比较 `GetProcessTimes` 得到的创建时间；两者不成立时按「未运行」处理，绝不误杀。
- **不做日志收集。** 服务的标准流全部丢弃。需要日志请让命令自己重定向，例如 `command: './server >> /var/log/web.log 2>&1'`。
- **不做健康检查、自动重启、systemd / launchd / Windows Service 注册。**
- **不做并发写保护。** 假设同一时刻只有一个 `lsm` 进程在写配置。
- **`ps` 只显示配置中登记的服务**，配置里已删除但 PID 文件残留的服务不可见。
- Windows 上 `cmd.exe` 的引号剥离规则特殊，命令中含复杂引号时请按 `cmd.exe` 的规则书写。
- **要求 Go 1.24+。** Go 1.22 链接器生成的 macOS arm64 二进制缺少 `LC_UUID`，在新版 macOS 上会被 `dyld` 拒绝加载；Go 1.24 起恢复写入该 load command。

---

## 开发

```sh
go vet ./...       # 静态检查
go test ./...      # 单元测试 + tests/ 集成测试（会构建真实二进制并启动真实进程）
go build ./...     # 构建
gofmt -l .         # 格式检查
```

CI 在 `ubuntu-latest`、`macos-latest`、`windows-latest` 上运行 `go vet` / `go test` / `go build`，另外还有 `-race`、`gofmt` 检查和 `linux/darwin/windows × amd64/arm64` 的交叉编译任务。

技术方案见 [dev.md](dev.md)。
