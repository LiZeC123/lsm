# Changelog

本文件记录 lsm 的所有重要变更，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.0]

首个版本，实现 dev.md 中的 M1–M6 里程碑。

### Added

- 命令行：`start`、`stop`、`restart`、`ps`、`list`、`add`、`del`，全局参数 `--config`。
- 配置：YAML 格式（`services.<name>.command` / `work_dir`），服务名与字段校验，
  原子写入（临时文件 + `fsync` + `rename`），写回时统一正斜杠。
- 配置路径：`--config` → `LSM_CONFIG` → `os.UserConfigDir()/lsm/config.yaml`。
- 进程管理（Unix）：`/bin/sh -c` 启动、`Setsid` 脱离终端、标准流丢弃、
  150ms 启动存活确认、向进程组发 `SIGTERM` 后按需升级 `SIGKILL`。
- 进程管理（Windows）：`cmd /S /C` + `DETACHED_PROCESS` 启动、
  `taskkill /PID <pid> /T /F` 停止、`OpenProcess` 存活检查。
- PID 文件：JSON 格式，位于配置同级 `run/` 目录（目录 `0700`、文件 `0600`），
  记录 pid、启动时刻、工作目录与命令。
- 陈旧 PID 文件自愈：`ps` 显示 `stopped` 且不修改文件，`start` 直接覆盖，
  `stop` 删除文件并报告未运行。
- PID 复用保护：Unix 仅对「PGID 等于 PID」的进程组发信号；
  Windows 通过进程创建时间与记录的启动时刻比对。
- 交互式 `add`：逐项提问、逐项校验并在失败时重新提问，相对工作目录自动解析为绝对路径。
- 交互式 `del`：删除前确认；服务运行中时拒绝删除，除非给出 `--force`（先停止再删除）。
- 退出码约定（0–8）与 `LSM_DEBUG=1` 错误链输出。
- 测试：各模块单元测试、以真实进程为对象的生命周期测试、覆盖完整交互序列的
  `tests/` 集成测试；GitHub Actions 三平台矩阵 + `-race` + `gofmt` + 交叉编译。
- 文档：README（安装、用法、配置、退出码、限制）。

### Notes

- 被管理的服务的标准流全部丢弃，不提供日志收集。
- 执行完立即退出的「一次性任务」不适合用 lsm 管理（见 README 的 150ms 说明）。
- 不做并发写保护，假设同一时刻只有一个 lsm 进程写配置。
- `ps` 只显示配置中登记的服务，配置已删除但 PID 文件残留的服务不可见。

[Unreleased]: https://github.com/LiZeC123/lsm/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/LiZeC123/lsm/releases/tag/v0.1.0
