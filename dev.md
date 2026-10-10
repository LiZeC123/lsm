# lsm 技术方案文档

**Lsm iS Manager — 跨平台后台服务管理 CLI**

版本：v0.1（设计稿）
目标平台：Linux / macOS / Windows
语言：Go 1.22+

---

## 1. 项目概述与设计目标

`lsm` 是一个跨平台的命令行后台服务管理工具。用户通过一个 YAML 配置文件登记若干“服务”（每个服务由名称、启动命令、工作目录定义），并通过 `lsm` 的子命令对这些服务进行启动、停止、重启、查看状态、增删配置等操作。

### 1.1 设计目标

1. **单文件二进制，零依赖运行。** 用户下载即用，无需安装运行时。
2. **跨平台一致体验。** Linux / macOS / Windows 上命令、输出、配置文件结构一致。
3. **启动即托管，管理程序不常驻。** `lsm start` 执行完毕后自身退出，被管理的服务以脱离终端的后台进程形式继续运行。
4. **命令字符串原样透传。** 用户提供的启动命令由系统 shell 解析，`lsm` 不尝试解析或重写命令字符串，避免转义地狱。
5. **配置可读可手改。** 配置文件为 YAML，用户可直接编辑；`lsm` 写回时不保证保留注释。
6. **输出干净。** 被管理服务的 stdin / stdout / stderr 全部丢弃，不污染 `lsm` 自身的输出。

### 1.2 非目标（本期不做）

- 不做并发写入保护（假设同一时刻只有一个 `lsm` 进程在写配置）。
- 不做服务健康检查、自动重启、日志收集。
- 不做非交互式 `add`（不提供 `--command` 之类的参数）。
- 不做 JSON 输出、脚本化查询接口。
- 不做每个服务的自定义环境变量（后续可扩展）。
- 不注册为 systemd / launchd / Windows Service。

---

## 2. 总体架构与模块划分

```
┌──────────────────────────────────────────────────┐
│                      lsm CLI                      │
│  (cobra 命令层: start/stop/restart/ps/list/add/del)│
└───────────────────────┬──────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   config     │ │   service    │ │   process    │
│  配置加载/保存 │ │  业务编排逻辑 │ │  平台相关进程 │
│  路径解析     │ │              │ │  启动/停止/检查│
└──────────────┘ └──────┬───────┘ └──────┬───────┘
                        │                │
                        ▼                ▼
                 ┌──────────────┐ ┌──────────────┐
                 │  ui (表格输出) │ │  pidfile     │
                 └──────────────┘ │  指纹校验     │
                                  └──────────────┘
```

### 2.1 模块职责

| 模块               | 职责                                                                      |
| ------------------ | ------------------------------------------------------------------------- |
| `cmd/`             | cobra 命令定义、参数解析、用户交互、退出码映射                            |
| `internal/config`  | YAML Schema、加载、校验、原子保存、跨平台路径解析                         |
| `internal/service` | 服务级业务编排：启动、停止、重启、状态查询、增删                          |
| `internal/process` | 平台相关：进程启动（后台化）、停止（信号/进程树）、存活检查、PID 文件读写 |
| `internal/ui`      | 表格输出、交互式问答、确认提示                                            |

---

## 3. 配置文件 Schema

### 3.1 结构定义

```yaml
# lsm 配置文件
services:
  <service-name>:
    command: <string>      # 必填，完整 shell 命令
    work_dir: <string>     # 必填，服务工作目录（绝对路径，正斜杠）
```

### 3.2 示例

```yaml
services:
  web:
    command: 'node /home/user/web/server.js --port 3000'
    work_dir: '/home/user/web'
  api:
    command: './bin/api -config ./conf/api.yaml'
    work_dir: '/opt/services/api'
  worker:
    command: 'python3 -u worker.py --queue default'
    work_dir: '/opt/services/worker'
```

### 3.3 字段规则

| 字段       | 类型   | 规则                                                       |
| ---------- | ------ | ---------------------------------------------------------- |
| `services` | map    | 顶层键，键为服务名                                         |
| 服务名     | string | 唯一，大小写敏感，匹配 `^[A-Za-z0-9_-]+$`，长度 1–64       |
| `command`  | string | 非空，原样存储                                             |
| `work_dir` | string | 非空，绝对路径，存储时统一转换为正斜杠；加载时只校验格式，目录是否存在由 start/restart（退出码 8）与 add（退出码 6）负责 |

### 3.4 序列化约定

- 使用 `gopkg.in/yaml.v3` 的 `yaml.Marshal` 输出，**所有字符串由库自动决定引号风格**，代码中不手动拼接 YAML 文本。
- 存储路径时统一转正斜杠：`filepath.ToSlash()`。Windows 上 `os/exec` 与文件系统 API 都接受正斜杠。
- 写文件采用**原子写入**：先写同目录临时文件，`fsync`，再 `os.Rename` 覆盖目标。防止进程崩溃导致配置截断。

---

## 4. 跨平台目录规范

### 4.1 配置目录

使用 Go 标准库 `os.UserConfigDir()`，它在各平台返回：

| 平台    | 返回值                            | lsm 最终路径                                    |
| ------- | --------------------------------- | ----------------------------------------------- |
| Linux   | `$XDG_CONFIG_HOME` 或 `~/.config` | `~/.config/lsm/config.yaml`                     |
| macOS   | `~/Library/Application Support`   | `~/Library/Application Support/lsm/config.yaml` |
| Windows | `%AppData%`                       | `%AppData%\lsm\config.yaml`                     |

### 4.2 覆盖机制

优先级从高到低：

1. 命令行参数 `--config <path>`
2. 环境变量 `LSM_CONFIG`
3. 平台默认路径

### 4.3 运行时目录

PID 文件存放在配置目录下的 `run/` 子目录：

```
<configDir>/lsm/
├── config.yaml
└── run/
    ├── web.pid
    ├── api.pid
    └── worker.pid
```

`run/` 目录在首次需要时按需创建，权限 `0700`。

---

## 5. 命令规范

### 5.1 命令一览

| 命令                 | 语义                   |
| -------------------- | ---------------------- |
| `lsm start <name>`   | 启动指定服务           |
| `lsm stop <name>`    | 停止指定服务           |
| `lsm restart <name>` | 重启指定服务           |
| `lsm ps`             | 列出所有服务的运行状态 |
| `lsm list`           | 列出所有已配置服务     |
| `lsm add`            | 交互式添加服务         |
| `lsm del <name>`     | 删除服务配置           |

全局参数：`--config <path>`。

### 5.2 start

**前置校验**：服务名存在；`work_dir` 存在且为目录；服务当前未运行。

**行为**：以 `work_dir` 为工作目录，通过系统 shell 启动 `command`，后台化，丢弃标准流，写 PID 文件，短暂确认进程存活。

**边界**：

- 服务不存在 → 退出码 3，stderr：`service 'xxx' not found`
- 服务已在运行 → 退出码 4，stderr：`service 'xxx' is already running (pid N)`
- 工作目录不存在 → 退出码 8，stderr 说明原因
- 启动后立即退出 → 退出码 8，stderr：`service 'xxx' exited immediately after start`

**输出**：`service 'xxx' started (pid N)`

### 5.3 stop

**边界**：

- 服务不存在 → 退出码 3
- 服务未运行 → 退出码 0，stdout：`service 'xxx' is not running`

**行为**：读取 PID 文件，发送终止信号（见 §6.2），删除 PID 文件。

**输出**：`service 'xxx' stopped`

### 5.4 restart

等价于：若运行中则 stop 后 start；若未运行则直接 start。边界行为沿用 start / stop 的规则。

### 5.5 ps

纯文本表格，列为：`NAME`、`STATUS`、`PID`、`UPTIME`。

```
NAME      STATUS     PID      UPTIME
web       running    12345    2h13m
api       stopped    -        -
worker    running    12401    8m
```

UPTIME 基于 PID 文件中记录的启动时刻计算。

### 5.6 list

纯文本表格，列为：`NAME`、`COMMAND`、`WORK_DIR`。

```
NAME      COMMAND                                       WORK_DIR
web       node /home/user/web/server.js --port 3000     /home/user/web
api       ./bin/api -config ./conf/api.yaml              /opt/services/api
```

### 5.7 add（仅交互式）

依次提问：

```
Service name: web
Command: node /home/user/web/server.js --port 3000
Working directory: /home/user/web
Service 'web' added.
```

校验项：

- 服务名非空、唯一、符合字符规则
- 命令非空
- 工作目录非空、存在、是目录

任一项失败给出提示并重新询问，或中止（用户 Ctrl+C）。

### 5.8 del

交互式确认：

```
$ lsm del web
Delete service 'web'? [y/N]: y
Service 'web' deleted.
```

若服务正在运行：

```
$ lsm del web
Service 'web' is running (pid 12345). Stop it first, or use --force.
```

带 `--force`：

```
$ lsm del web --force
Stopping service 'web' (pid 12345)...
Delete service 'web'? [y/N]: y
Service 'web' deleted.
```

删除时同时删除 `run/<name>.pid`（若存在）。

---

## 6. 后台化与进程管理设计

### 6.1 启动（Unix）

```go
cmd := exec.Command("/bin/sh", "-c", svc.Command)
cmd.Dir = svc.WorkDir
cmd.Stdin = nil    // 连接到 /dev/null
cmd.Stdout = nil   // 连接到 /dev/null
cmd.Stderr = nil   // 连接到 /dev/null
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setsid: true,  // 新建会话，脱离控制终端
}
```

**关键说明**：

- `command` 作为 `argv[2]` 独立传入，**不经过任何外层 shell 解析**。命令字符串内部的双引号、单引号、管道、重定向、`$VAR` 全部由 `/bin/sh` 自己解析，与 `lsm` 无关。
- `Setsid: true` 使子进程成为新会话首进程，其 PGID = 子进程 PID，脱离父进程的终端，父进程退出后不会收到 SIGHUP。
- `Stdin/Stdout/Stderr` 为 `nil` 时，Go 的 `os/exec` 会将其连接到 `/dev/null`，实现标准流丢弃。

### 6.2 启动（Windows）

```go
cmd := exec.Command("cmd", "/S", "/C", svc.Command)
cmd.Dir = svc.WorkDir
cmd.Stdin = nil
cmd.Stdout = nil
cmd.Stderr = nil
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
}
// const detachedProcess = 0x00000008  // DETACHED_PROCESS，Go syscall 未导出
```

**关键说明**：

- 使用 `cmd /S /C` 形式而非 `cmd /C`，`/S` 让 cmd.exe 采用更可预测的引号剥离规则。
- `DETACHED_PROCESS` 让子进程不继承父进程的控制台。
- Windows 上 `os/exec` 最终把参数拼成命令行字符串交给 `CreateProcess`。由于 `cmd.exe` 的历史遗留解析规则特殊，命令字符串中若含复杂引号，建议用户参考 `cmd.exe` 文档书写。

### 6.3 启动后存活确认

由于使用 shell 启动，如果用户命令的可执行文件不存在，`cmd.Start()` 仍会成功（因为 `sh`/`cmd` 本身存在），错误会发生在 shell 内部并被丢弃到 `/dev/null`，用户看不到。

处理方式：`Start()` 成功后短暂等待（150ms），然后用 `kill(pid, 0)`（Unix）/ `OpenProcess`（Windows）检查进程是否存活。若已退出，视为启动失败，返回错误，不写 PID 文件。

**权衡**：某些极快的服务可能在 150ms 内正常启动、执行完任务、退出。这类“一次性任务”不适合用 `lsm` 管理，属预期之外的使用场景。150ms 阈值会在文档中说明。

### 6.4 停止（Unix）

1. 从 PID 文件读取 PID。
2. 校验进程存在（见 §7）。
3. 向**进程组**发送 `SIGTERM`：`syscall.Kill(-pgid, syscall.SIGTERM)`，其中 `pgid == pid`（因启动时使用了 `Setsid`）。
4. 轮询等待进程组消失，最多 10 秒。
5. 超时未退出，发送 `SIGKILL`：`syscall.Kill(-pgid, syscall.SIGKILL)`。
6. 删除 PID 文件。

### 6.5 停止（Windows）

调用外部命令：

```
taskkill /PID <pid> /T /F
```

- `/T` 终止整个进程树。
- `/F` 强制终止。由于目标是 detached 控制台进程，`WM_CLOSE` 消息方式不可靠，使用 `/F`。

随后删除 PID 文件。

### 6.6 进程存活检查

- **Unix**：`syscall.Kill(pid, 0)`。返回 `nil` 或 `EPERM` 视为存在；返回 `ESRCH` 视为不存在。
- **Windows**：`OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid)`，成功则存在。

---

## 7. PID 文件与进程指纹校验

### 7.1 PID 文件格式

使用 JSON，便于扩展和人工查看。文件名 `<name>.pid`，位于 `<configDir>/lsm/run/`。

```json
{
  "pid": 12345,
  "started_at": 1712345678,
  "work_dir": "/home/user/web",
  "command": "node /home/user/web/server.js --port 3000"
}
```

字段说明：

| 字段         | 说明                                     |
| ------------ | ---------------------------------------- |
| `pid`        | 进程 ID                                  |
| `started_at` | `lsm start` 成功时的 Unix 时间戳（秒）   |
| `work_dir`   | 启动时的工作目录                         |
| `command`    | 启动时的命令字符串（冗余存储，便于诊断） |

### 7.2 存活判定策略（分层）

**基础判定（所有平台）**：

1. PID 文件存在。
2. 进程 PID 存在（§6.6）。
3. 两者都满足 → 视为“运行中”。

**增强判定（Linux，可选实现）**：

读取 `/proc/<pid>/stat` 的第 22 个字段（`starttime`，单位 clock tick），与 `started_at` 做归一化比较（需结合 `/proc/stat` 的 `btime`），可精确识别 PID 复用。本项作为 P1 增强，不阻塞 v0.1。

**macOS / Windows**：v0.1 仅使用基础判定。后续可通过 `sysctl KERN_PROC` / `GetProcessTimes` 补充。

### 7.3 陈旧 PID 文件处理

若 PID 文件存在但进程已不存在，视为陈旧文件：

- `ps` 显示 `stopped`。
- `start` 时忽略陈旧文件并覆盖。
- `stop` 时删除陈旧文件并提示 `not running`。

---

## 8. Go 包结构与关键接口

### 8.1 目录结构

```
lsm/
├── main.go
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go
│   ├── start.go
│   ├── stop.go
│   ├── restart.go
│   ├── ps.go
│   ├── list.go
│   ├── add.go
│   └── del.go
├── internal/
│   ├── config/
│   │   ├── schema.go      // Config / Service 结构
│   │   ├── load.go        // 加载 + 校验
│   │   ├── save.go        // 原子保存
│   │   └── paths.go       // 跨平台路径解析
│   ├── service/
│   │   ├── manager.go     // 编排：Start / Stop / Restart / Delete / Add
│   │   └── status.go      // 状态查询与聚合
│   ├── process/
│   │   ├── launch_unix.go
│   │   ├── launch_windows.go
│   │   ├── stop_unix.go
│   │   ├── stop_windows.go
│   │   ├── alive_unix.go
│   │   ├── alive_windows.go
│   │   └── pidfile.go     // 通用：读写 JSON PID 文件
│   └── ui/
│       ├── table.go       // tabwriter 表格
│       └── prompt.go      // 交互式问答
└── tests/
    └── integration_test.go
```

### 8.2 核心类型

```go
// config/schema.go
type Config struct {
    Services map[string]Service `yaml:"services"`
}

type Service struct {
    Command string `yaml:"command"`
    WorkDir string `yaml:"work_dir"`
}

// process/pidfile.go
type PIDInfo struct {
    PID       int    `json:"pid"`
    StartedAt int64  `json:"started_at"`
    WorkDir   string `json:"work_dir"`
    Command   string `json:"command"`
}
```

### 8.3 关键接口

```go
// config
func Load(path string) (*Config, error)
func (c *Config) Save(path string) error
func DefaultPath() (string, error)

// process
func Launch(svcName string, svc config.Service, pidPath string) (int, error)
func Stop(pidPath string) error
func IsAlive(info PIDInfo) bool
func ReadPIDFile(path string) (*PIDInfo, error)
func WritePIDFile(path string, info PIDInfo) error
func RemovePIDFile(path string) error

// service
func (m *Manager) Start(name string) error
func (m *Manager) Stop(name string) error
func (m *Manager) Restart(name string) error
func (m *Manager) Status() ([]ServiceStatus, error)
```

### 8.4 构建标签

平台相关文件通过文件名后缀区分（`_unix.go` / `_windows.go`）。`_unix.go` 使用 `//go:build !windows`，`_windows.go` 使用 `//go:build windows`。`unix` 覆盖 Linux 与 macOS。

---

## 9. 错误处理与退出码约定

### 9.1 退出码

| 码  | 含义                                   |
| --- | -------------------------------------- |
| 0   | 成功                                   |
| 1   | 通用错误（未分类）                     |
| 2   | 命令行用法错误（cobra 默认）           |
| 3   | 服务不存在                             |
| 4   | 服务已在运行                           |
| 5   | 服务未运行（仅用于需要此语义的场景）   |
| 6   | 配置读取 / 解析 / 校验失败             |
| 7   | 配置写入失败                           |
| 8   | 进程操作失败（启动 / 停止 / 存活检查） |

### 9.2 输出约定

- **错误输出**：stderr，格式 `lsm: <message>`，不含堆栈（用户不需要）。
- **正常输出**：stdout。
- **详细调试**：`LSM_DEBUG=1` 时额外输出诊断信息到 stderr。

### 9.3 错误封装

内部使用 `fmt.Errorf("...: %w", err)` 包装底层错误。`cmd` 层统一在最外层捕获，映射到退出码，并打印简洁消息。

---

## 10. 测试方案与 CI

### 10.1 单元测试

| 模块      | 覆盖点                                                                                                                                                 |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `config`  | Load / Save round-trip；YAML 含特殊字符（引号、反斜杠、管道）的字符串保存与加载；路径正斜杠归一化；Schema 校验（服务名非法、字段为空、work_dir 非绝对路径）；`CheckWorkDir` 对不存在 / 非目录的判定 |
| `process` | PID 文件读写；`IsAlive` 对已退出 / 不存在 PID 的判定；Unix 平台模拟进程组停止                                                                          |
| `service` | Start 已运行服务的报错路径；Stop 未运行服务返回成功；Restart 分支覆盖                                                                                  |

### 10.2 集成测试

在 Linux 与 macOS 上使用 `sleep 300` 作为测试服务：

1. `lsm add` 交互通过 stdin 注入。
2. `lsm list` 确认输出。
3. `lsm start test-svc` → `lsm ps` 显示 running → `lsm stop test-svc` → `lsm ps` 显示 stopped。
4. `lsm del test-svc --force` 清理。

Windows 集成测试可选用 `timeout /t 300` 或 `ping -n 300 127.0.0.1`。

### 10.3 CI（GitHub Actions）

- 矩阵：`ubuntu-latest`、`macos-latest`、`windows-latest`。
- 步骤：`go vet` → `go test ./...` → `go build`。
- 额外任务：`GOOS=linux/darwin/windows GOARCH=amd64/arm64 go build`，确保跨平台编译通过。

### 10.4 手工验收清单

- 三平台安装后，`lsm` 无参数输出 help。
- 首次运行 `lsm list` 在空配置下输出空表格而非报错。
- 手改配置文件引入 YAML 语法错误，`lsm list` 给出清晰错误。
- 工作目录含空格、命令含引号，start / stop 正常。

---

## 11. 开发里程碑

| 里程碑                     | 内容                                                 | 交付标准                                        |
| -------------------------- | ---------------------------------------------------- | ----------------------------------------------- |
| **M1：骨架与配置**         | `go.mod`、cobra 根命令、`config` 模块、`list` 子命令 | `lsm list` 能读取并展示配置                     |
| **M2：启动与停止（Unix）** | `process` 模块 Unix 实现、`start` / `stop`、PID 文件 | Linux/macOS 上能 start / stop 一个 `sleep` 服务 |
| **M3：状态与重启**         | `ps`、`restart`、存活检查、陈旧 PID 处理             | `ps` 显示正确状态；`restart` 正常               |
| **M4：增删交互**           | `add` 交互、`del` 交互与 `--force`                   | 交互流程完整，校验清晰                          |
| **M5：Windows 支持**       | Windows 平台进程启动 / 停止 / 存活检查               | Windows 上能 start / stop 一个 `ping` 服务      |
| **M6：测试、CI、文档**     | 单元 + 集成测试、GitHub Actions、README、CHANGELOG   | CI 三平台全绿；README 覆盖安装与用法            |
| **M7（可选）：增强**       | Linux PID 复用精确校验、可选日志文件、`env:` 字段    | 按需推进                                        |

---

## 附录 A：配置示例（含特殊字符）

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

说明：

- 单引号包裹的 YAML 字符串不解析反斜杠，Windows 路径 `C:\...` 安全。
- `command` 中的 `"` 与 `|` 完全由 shell 处理，`lsm` 不介入。
- `work_dir` 统一以正斜杠存储。

## 附录 B：被否决的备选方案

| 备选                              | 否决原因                               |
| --------------------------------- | -------------------------------------- |
| 自己解析命令字符串成 `[]string`   | 转义规则复杂，必然出错                 |
| `add` 支持 `--command` 非交互参数 | 命令行转义层级爆炸，用户体验差         |
| 管理程序常驻守护                  | 与“单文件 CLI”定位冲突，复杂度大幅上升 |
| TOML 配置                         | YAML 已可满足，且 YAML 生态更成熟      |
| JSON 输出                         | 当前无脚本化需求，徒增维护面           |