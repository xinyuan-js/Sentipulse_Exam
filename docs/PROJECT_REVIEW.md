# 项目结构审查与功能说明

本文基于 `docs/requirements.md` 对当前项目进行结构审查，明确已开发功能和示例插件能力，方便评审时快速核对设计质量、模块边界和完成度。

## 结构审查结论

当前项目采用标准 Go 工程组织方式：

```text
.
├── cmd/plugin-executor/        CLI 主程序入口，只负责进程启动
├── internal/app/               CLI 应用编排、watch 模式、输出处理
├── internal/config/            命令行参数解析和默认配置
├── internal/core/              Manager 生命周期协调
├── internal/executor/          并发执行器和进程插件适配器
├── internal/loader/            插件扫描和 manifest 校验
├── internal/model/             插件领域模型和状态模型
├── internal/registry/          插件注册表、启用禁用、依赖校验
├── internal/version/           版本约束解析和匹配
├── pkg/protocol/               主程序和插件共享的 JSON 协议
├── pkg/sdk/                    Go 插件辅助 SDK
├── plugins/                    运行时插件 manifest，模拟可独立维护的插件发布目录
├── examples/plugins/           示例插件源码，覆盖 Go、Python、JavaScript 插件形态
├── examples/inputs/            示例输入数据
├── docs/                       项目结构审查、模块流程图、功能映射和评估说明
├── README.md                   架构设计、运行方式和取舍说明
├── go.mod                      Go module 定义，要求 Go >= 1.20
└── Makefile                    常用测试和运行命令
```

这个结构把“插件系统框架”和“插件业务实现”拆开：

- 主程序入口只在 `cmd/plugin-executor`，不放业务逻辑。
- CLI 编排、配置解析、生命周期、加载、注册、执行、版本和模型分层放在 `internal/`。
- 对插件开发者可复用的协议和 Go SDK 放在 `pkg/`，避免外部插件引用 `internal/`。
- `plugins` 只放运行时描述文件，代表插件交付物；主程序通过 manifest 加载插件。
- `examples/plugins` 只放插件实现示例，证明插件可以独立开发和独立维护。
- `examples/inputs` 放示例输入，便于评审快速运行。
- `docs` 放评审材料、模块流程图和维护说明，不和运行时代码混在一起。

模块化设计细节、加载流程图、执行流程图、热加载流程图和异常降级流程图见 `docs/architecture.md`。

## 原始要求与实现映射

| 原始要求 | 当前实现 | 关键文件 |
| --- | --- | --- |
| Go 实现，Go >= 1.20 | 使用 Go module，`go.mod` 声明 `go 1.20` | `go.mod` |
| 统一插件规范 | 使用 `Plugin` 接口和 stdin/stdout JSON 协议，插件实现只需遵守协议 | `internal/model/types.go`, `pkg/protocol/protocol.go` |
| 从指定目录加载插件 | 递归扫描 `plugin.json`，解析 manifest | `internal/loader/loader.go`, `internal/loader/manifest.go` |
| 主程序不依赖具体插件实现 | 主程序只依赖 manifest 和进程协议，不 import 示例插件 | `cmd/plugin-executor/main.go`, `internal/executor/process.go` |
| 插件启用/禁用 | manifest 支持 `enabled`，CLI 支持 `-enable`、`-disable` | `internal/registry/registry.go`, `internal/app/app.go` |
| 插件异常不导致主程序崩溃 | 单插件错误、协议错误、非零退出、panic 均隔离为执行结果错误 | `internal/executor/executor.go`, `internal/executor/process.go` |
| 获取名称、版本、状态 | `Registry.States()` 返回元信息、启用状态、异常原因、依赖信息 | `internal/model/types.go`, `internal/registry/registry.go` |
| 执行已启用插件并汇总结果 | `Executor` 并发执行所有状态正常的启用插件，返回执行汇总 | `internal/executor/executor.go` |
| 热加载/热卸载 | `Manager.Reload()` 和 `Manager.Watch()` 重新扫描目录并替换注册表，未变化插件复用，配置变更或删除 manifest 时关闭旧适配器 | `internal/core/manager.go`, `internal/registry/registry.go`, `internal/core/manager_test.go` |
| 执行超时控制 | 每个插件 manifest 可配置 `timeout`，超时后终止插件进程 | `internal/executor/process.go` |
| 插件隔离方案 | 插件以独立 OS 进程运行，无共享内存，进程失败不会拖垮主程序 | `internal/executor/process.go`, `internal/executor/process_unix.go`, `internal/executor/process_windows.go` |
| 依赖关系与版本约束 | manifest 的 `depends_on` 支持依赖 ID 和 `=`, `>`, `>=`, `<`, `<=` 版本约束 | `internal/registry/registry.go`, `internal/version/version.go` |
| 失败隔离与降级策略 | 单插件失败只影响自身；配置 `fallback` 时返回降级结果并标记 `degraded` | `internal/executor/executor.go`, `plugins/go_slow/plugin.json` |
| 多类型插件支持 | 同一协议支持 Go、Python、JavaScript 等外部进程插件 | `examples/plugins/`, `plugins/` |
| README 设计说明 | README 包含架构、核心设计、取舍、后续演进和第三方库说明 | `README.md` |

## 已开发功能清单

1. 插件 manifest 加载
   - 支持指定插件目录。
   - 支持递归扫描多个 `plugin.json`。
   - 支持 manifest 字段校验、依赖版本约束校验、尾随 JSON 拒绝和重复插件 ID 检测。

2. 插件注册与状态管理
   - 维护插件元信息、启用状态、异常状态、依赖信息和加载时间。
   - 支持运行时启用/禁用。
   - 支持插件异常状态记录。

3. 插件执行
   - 主程序读取 JSON 输入。
   - 只执行已启用且依赖满足的插件。
   - 支持并发执行和并发度控制。
   - 汇总每个插件的结果、错误、耗时和降级标记。

4. 生命周期管理
   - 支持重新加载插件目录。
   - 支持 watch 模式周期扫描，实现热加载和热卸载。
   - 未变化插件复用已有适配器，配置变更或删除插件时调用插件关闭逻辑。

5. 超时与隔离
   - 每个插件使用独立超时上下文。
   - 插件进程超时会被终止。
   - Unix 平台按进程组终止，降低子进程残留风险。

6. 依赖与版本约束
   - 必需依赖缺失、禁用、异常或版本不满足时，依赖方不会执行。
   - 支持检测依赖环并标记异常。

7. 失败隔离与降级
   - 插件执行失败不会中断其他插件。
   - 插件可通过 manifest 配置静态 fallback。
   - 输出汇总中保留真实错误和降级结果。

8. 多语言插件接入
   - 插件只需从 stdin 读取请求 JSON，并向 stdout 输出响应 JSON。
   - 示例覆盖 Go、Python、JavaScript。

## 已实现插件功能说明

| 插件 ID | 默认状态 | 类型 | 功能 | 设计目的 |
| --- | --- | --- | --- | --- |
| `go.echo` | 启用 | Go 进程插件 | 原样回显输入数据，并返回插件 ID 与处理时间 | 验证最基础的数据处理与 Go 插件接入 |
| `go.word_stats` | 启用 | Go 进程插件 | 读取输入中的 `text`，统计字符数、单词数和是否为空 | 验证业务处理插件、依赖关系和版本约束 |
| `go.slow` | 启用 | Go 进程插件 | 故意睡眠超过 manifest 超时时间 | 验证超时控制、失败隔离和 fallback 降级 |
| `python.uppercase` | 禁用 | Python 进程插件 | 读取 `text` 并输出大写文本和长度 | 证明 Python 插件可通过同一协议接入 |
| `js.reverse` | 禁用 | JavaScript 进程插件 | 读取 `text` 并输出反转文本和长度 | 证明 JavaScript 插件可通过同一协议接入 |

默认只启用 Go 示例，避免评估环境缺少 Python 或 Node 时影响基础运行。可通过 CLI 临时启用：

```bash
go run ./cmd/plugin-executor -plugins ./plugins -enable python.uppercase
go run ./cmd/plugin-executor -plugins ./plugins -enable js.reverse
```

## 结构规范建议

当前结构已经满足交付要求。后续如果继续扩展，建议保持以下规则：

- 新增主程序入口放入 `cmd/<name>/`。
- 新增框架能力按职责放入 `internal/app`、`internal/core`、`internal/loader`、`internal/registry`、`internal/executor` 等内部包，不把 CLI 入口逻辑和运行时核心逻辑混在一起。
- 新增插件发布配置放入 `plugins/<plugin-id>/plugin.json`。
- 新增插件源码放入 `examples/plugins/<plugin-id>/` 或独立仓库，主程序不得 import 插件源码。
- 新增说明文档放入 `docs/`，README 只保留入口级说明和关键索引。

## 评估关注点回应

- 模块边界：入口、加载、注册、执行、进程协议、版本约束、文档均有独立边界。
- 解耦性：主程序通过 manifest 和 JSON 协议调用插件，不绑定语言、不绑定源码。
- 异常处理：加载失败、依赖失败、执行失败、超时、非零退出、协议错误和尾随 stdout 均被状态化或结果化。
- 长期维护：插件协议稳定，未来可以替换热加载源、执行调度或隔离机制，而不影响业务插件协议。
