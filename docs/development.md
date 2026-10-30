# cereals 本地开发与构建

> **这是一个 vibecoding 项目，主要通过 AI 辅助编码构建。介意勿用。**
>
> **本项目面向在本地电脑通过 kubeconfig 连接远端 Kubernetes 集群的运维人员、开发者和集群管理员。**

本文面向从源码运行、修改或构建 cereals 的用户与贡献者。应用使用本机 Go 后端访问远端 Kubernetes API，前端通过 Wails 生成的接口调用后端。

## 环境准备

| 工具             | 要求                                                                               |
| ---------------- | ---------------------------------------------------------------------------------- |
| Go               | 1.26 或更高的兼容版本，以 `go.mod` 为准                                            |
| Node.js          | 22.15+，同时安装 npm                                                               |
| Wails            | CLI 与 Go 运行时固定为 v3.0.0                                                      |
| macOS 原生工具   | Command Line Tools；当前应用包最低系统版本为 macOS 15.0                            |
| Windows 原生环境 | WebView2 Runtime，以及对应平台的 Wails 构建依赖                                    |
| 安装器打包       | 当前脚本在 macOS 上生成本机架构 DMG 与 Windows amd64 安装器；需 NSIS 的 `makensis` |

各平台的原生依赖可参考 [Wails 安装说明](https://wails.io/getting-started/installation/)。建议使用与项目相同的 Go 工具链安装 Wails CLI；升级 Go 后若绑定生成出现源码解析版本警告，使用当前工具链重新安装同版本 CLI。

```sh
git clone https://github.com/cereals-af/cereals-desktop.git
cd cereals-desktop

go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
go mod download
npm --prefix frontend ci
```

确认 `wails3` 可以从终端直接执行。默认安装目录是 `go env GOPATH` 对应目录下的 `bin`；设置过 `GOBIN` 时，以该目录为准。

## 开发运行

在仓库根目录执行：

```sh
wails3 task dev
```

该任务编译 Go 程序、启动前端热更新，并打开原生桌面窗口。普通开发界面仍会通过本机后端访问实际集群，连接与操作前请确认目标。

需要隔离开发设置时，可以指定配置目录，例如在 macOS 终端中：

```sh
CEREALS_CONFIG_DIR="$PWD/.test-env/config" wails3 task dev
```

`.test-env/` 已被 Git 忽略。隔离设置目录不会自动替换 kubeconfig，请手工导入用于开发验证的配置文件。

后端服务签名变更后，使用以下命令重新生成绑定：

```sh
wails3 task bindings
```

不要手工修改 `frontend/bindings/` 中的生成文件。

## 生产构建与安装包

```sh
# 当前平台生产程序，输出到 bin/cereals 或 bin/cereals.exe
wails3 task build

# 运行上述程序
wails3 task run

# 仅限 macOS：生成 bin/cereals.app 本地应用包
wails3 task package:macos

# 仅限 macOS：生成本机架构 DMG、Windows amd64 安装器和更新附件
wails3 task package:installers
```

`package:macos` 用于本地应用包组装；`package:installers` 的 macOS 流程额外执行本地 ad-hoc 签名。正式分发仍需自行配置所需的签名与公证。

Windows 安装器打包需要 `makensis`，并使用 Wails 提供的 WebView2 bootstrapper。目标机器缺少 WebView2 Runtime 时，安装器需要联网安装；离线环境应先准备运行时。

| 输出位置          | 内容                                         |
| ----------------- | -------------------------------------------- |
| `bin/`            | 桌面程序与本地应用包                         |
| `bin/installers/` | DMG、Windows 安装器及各自的 SHA-256 校验文件 |
| `bin/updates/`    | 专用更新附件与更新器要求的 `SHA256SUMS`      |

版本取自 `frontend/package.json`，构建任务将其注入 Go 版本信息，打包脚本使用同一版本生成文件名和平台元数据。修改发行版本时还需同步锁文件、`build/config.yml` 及 Windows manifest，详见 [更新与发布](updates.md)。

开发构建不更新自身。macOS 的自动更新仅支持标准 `.app` 包，直接运行 `bin/cereals` 时不启用自动更新。

## 检查与测试

首次检查前，先生成绑定和 Go 需要嵌入的前端静态文件：

```sh
wails3 task frontend
wails3 task check
```

`check` 包含 Go vet、`internal` 与更新校验工具的竞态测试，以及前端 ESLint、Prettier、TypeScript 和 Vitest。

前端交互测试使用 Playwright：

```sh
cd frontend
npx playwright install chromium
npm run test:e2e
```

Playwright 使用仅在开发模式启用的模拟 API，不等价于真实集群验证，也不能代替原生桌面窗口、输入法、剪贴板和多窗口检查。

需要验证 API Server 行为时，准备 envtest 的 `kube-apiserver` 和 `etcd` 二进制，并在仓库根目录执行：

```sh
KUBEBUILDER_ASSETS=/path/to/envtest/bin wails3 task test:integration
```

该任务启动隔离 API Server，不读取用户 kubeconfig。真实控制器、CSI、认证插件、生产 Webhook 及操作系统安装行为，应在相应环境另行验证。提交说明只列出实际运行过的检查及结果。

## 代码结构

```text
main.go                 应用装配与原生窗口
internal/appupdate/     应用内更新与发行附件校验
internal/buildinfo/     构建版本信息
internal/cluster/       kubeconfig 导入与连接生命周期
internal/kube/          Kubernetes API 适配
internal/manifest/      YAML、预检、差异与写入保护
internal/workload/      工作负载业务逻辑
internal/session/       Pod 日志、终端与端口转发
internal/filetransfer/  可取消的容器文件传输
internal/localstore/    本地配置与兼容处理
internal/desktop/       桌面集成与 Wails 绑定服务
internal/integration/   隔离 API Server 集成测试
frontend/src/           React 页面、组件、Hook 与数据适配
frontend/bindings/      自动生成的 TypeScript 绑定
frontend/e2e/           前端交互测试
build/                  平台配置、图标和安装器模板
tools/                  打包、校验与图标生成工具
docs/                   使用、开发构建和更新发布说明
```

## 提交约定

提交应围绕一个明确目的组织，避免混入无关格式化、依赖升级和重构。界面修改检查实际渲染；Kubernetes 写入与流式能力需要覆盖相应权限、并发冲突和资源清理行为。

公开文档只包含面向用户和贡献者的说明。功能实施计划、过程笔记和历史验收记录保存在被 Git 忽略的 `.local-docs/`，不提交到仓库。安装包和构建产物位于 `bin/`，通过 GitHub Releases 分发，不加入源码提交。

不要提交 kubeconfig、凭据、Secret、个人环境文件或包含敏感内容的日志。报告问题或提交示例前，请先脱敏。
