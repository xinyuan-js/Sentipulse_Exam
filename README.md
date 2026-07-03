# 插件化执行系统

这是一个用 Go 实现的插件化执行系统。主程序只依赖统一的插件 manifest 和 JSON 执行协议，不依赖任何具体业务插件代码。插件以独立进程运行，因此 Go、Python、JavaScript 或其他语言只要遵守协议即可接入。

## 快速运行

```bash
go test ./...
go run ./cmd/plugin-executor -plugins ./plugins -input '{"text":"hello plugin system","value":42}'
go run ./cmd/plugin-executor -plugins ./plugins -list
```

如果当前环境不允许写默认 Go 构建缓存，可以把缓存放到项目内：

```bash
mkdir -p .cache/go-build
GOCACHE=$PWD/.cache/go-build go test ./...
GOCACHE=$PWD/.cache/go-build go run ./cmd/plugin-executor -plugins ./plugins
```

## 目录结构

```text
cmd/plugin-executor/        主程序入口和 CLI
internal/app/                CLI 应用编排、watch 模式、输出处理
internal/config/             命令行参数解析和默认配置
internal/core/               Manager 生命周期协调
internal/executor/           并发执行器和进程插件适配器
internal/loader/             插件扫描和 manifest 校验
internal/model/              插件领域模型和状态模型
internal/registry/           插件注册表、启用禁用、依赖校验
internal/version/            版本约束解析和匹配
pkg/protocol/                主程序和插件共享的 JSON 协议
pkg/sdk/                     Go 插件辅助 SDK
plugins/                    插件 manifest 示例目录
examples/plugins/           多语言插件示例实现
examples/inputs/            示例输入数据
docs/                       结构审查、功能映射和评估说明
docs/requirements.md        原始开发要求
```

更详细的项目结构审查、原始要求映射和插件功能说明见 [docs/PROJECT_REVIEW.md](docs/PROJECT_REVIEW.md)，模块职责和流程图见 [docs/architecture.md](docs/architecture.md)。

## 架构设计

系统分为四层：

1. `Loader` 从指定目录递归扫描 `plugin.json`，解析插件元信息、命令、超时、依赖和降级配置。
2. `Registry` 维护插件注册表，提供启用、禁用、状态查询、依赖和版本约束校验。
3. `Executor` 获取已启用且依赖满足的插件，并发执行，收集每个插件的结果、错误、耗时和降级状态。
4. `ProcessPlugin` 通过独立进程运行插件，把输入 JSON 写入 stdin，从 stdout 读取 JSON 结果。

核心边界是 manifest 和协议，而不是语言或包依赖。主程序不 import 任何业务插件实现，新增插件只需要新增目录和 manifest。

完整架构图、加载流程、执行流程、热加载流程、超时隔离流程和降级流程已整理在 [docs/architecture.md](docs/architecture.md)。

## 插件 manifest

示例：

```json
{
  "id": "go.word_stats",
  "name": "Go Word Stats",
  "version": "1.1.0",
  "type": "go-process",
  "enabled": true,
  "command": "go",
  "args": ["run", "./examples/plugins/go_word_stats"],
  "working_dir": "../..",
  "timeout": "10s",
  "depends_on": [
    { "id": "go.echo", "version": ">=1.0.0" }
  ],
  "fallback": {
    "message": "fallback result"
  }
}
```

字段说明：

- `id/name/version/type`：插件基础元信息。
- `enabled`：是否启用，默认启用。
- `command/args/working_dir`：插件进程启动方式，路径相对 manifest 所在目录解析。
- `timeout`：单插件执行超时，默认 `3s`。
- `depends_on`：依赖插件和版本约束，支持 `=`, `>`, `>=`, `<`, `<=`。
- `fallback`：插件失败或超时时返回的降级结果。

## 插件协议

主程序写入 stdin：

```json
{
  "data": {
    "text": "hello"
  }
}
```

插件从 stdout 返回：

```json
{
  "result": {
    "words": 1
  }
}
```

插件业务错误可返回：

```json
{
  "error": "invalid input"
}
```

stdout 必须只输出一个协议 JSON，日志应写入 stderr；主程序会拒绝未知响应字段和尾随 stdout 内容。

## 已实现能力

- 从指定目录加载插件，支持递归扫描多个 `plugin.json`。
- 插件启用、禁用和状态查询。
- 插件异常、协议错误、非零退出和 panic 隔离，不会导致主程序崩溃。
- 插件热加载、热卸载：`Manager.Watch` 或 CLI `-watch` 按周期重扫目录并替换注册表，删除 manifest 会卸载插件；CLI watch 每轮 reload 后会重新应用 `-enable` / `-disable` 运行期覆盖。
- 单插件执行超时控制，超时后终止插件进程。
- 进程隔离：插件运行在独立 OS 进程，主程序和插件无共享内存，插件崩溃只影响自己的执行结果。
- 依赖关系和版本约束校验，直接或传递依赖缺失、禁用、异常或版本不满足时不会执行该插件。
- 失败隔离与降级：单插件失败会记录错误；配置 `fallback` 时返回降级结果并标记 `degraded`。
- 多语言插件支持：示例包含 Go 插件，以及默认禁用的 Python、JavaScript 插件 manifest。

## 示例插件功能

| 插件 ID | 默认状态 | 功能 |
| --- | --- | --- |
| `go.echo` | 启用 | 回显输入数据，并返回插件 ID 与处理时间。 |
| `go.word_stats` | 启用 | 统计输入 `text` 的字符数、单词数和是否为空；依赖 `go.echo >= 1.0.0`。 |
| `go.slow` | 启用 | 故意超过超时时间，用于验证超时隔离和 fallback 降级。 |
| `python.uppercase` | 禁用 | 将输入 `text` 转为大写并返回长度，展示 Python 插件接入。 |
| `js.reverse` | 禁用 | 反转输入 `text` 并返回长度，展示 JavaScript 插件接入。 |

## 关键取舍

- 没有使用 Go 的 `plugin` 包。该方案平台限制较多，且会把插件实现和主程序 ABI、构建环境绑定得更紧。
- 没有引入完整插件框架。项目只使用 Go 标准库，便于评估代码本身的模块边界、生命周期和错误处理。
- 热加载采用轮询扫描。它比文件事件监听更简单可靠，不依赖平台特定能力；生产环境可替换为 fsnotify 类事件源，并继续复用现有 `Reload -> Registry.Replace` 生命周期逻辑。
- 隔离采用进程边界。当前实现负责进程、超时和环境变量隔离；更强资源隔离可以在 manifest 的 `command` 中接入容器、沙箱、cgroup 或受限用户。

## CLI 用法

```bash
# 执行所有启用且依赖满足的插件
go run ./cmd/plugin-executor -plugins ./plugins -input '{"text":"hello world"}'

# 使用示例输入文件执行
go run ./cmd/plugin-executor -plugins ./plugins -input-file ./examples/inputs/default.json

# 查看插件状态
go run ./cmd/plugin-executor -plugins ./plugins -list

# 仅本次运行禁用某个插件
go run ./cmd/plugin-executor -plugins ./plugins -disable go.slow

# 本次运行启用 Python 示例插件
go run ./cmd/plugin-executor -plugins ./plugins -enable python.uppercase

# 持续观察插件目录，演示热加载/热卸载
go run ./cmd/plugin-executor -plugins ./plugins -watch -interval 2s
```

也可以使用 Makefile：

```bash
make test
make vet
make run
make list
make watch
make run-multilang
```

## 第三方库说明

未使用第三方库。版本约束、加载、执行、超时、并发和进程管理均由标准库实现。

## 后续演进

当前实现已经覆盖开发要求。若进入生产场景，建议补充：

- manifest 签名校验和插件来源认证。
- 插件配额、内存和 CPU 限制。
- 插件执行审计日志和指标上报。
- 持久化的启用/禁用配置和管理 API。
