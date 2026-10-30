# cereals

> **这是一个 vibecoding 项目，主要通过 AI 辅助编码构建。介意勿用。**
>
> **本项目面向在本地电脑通过 kubeconfig 连接远端 Kubernetes 集群的运维人员、开发者和集群管理员。**

cereals 是一个开源 Kubernetes 桌面客户端，使用 **Wails 3、Go、React 和 TypeScript** 构建。应用运行在你的电脑上，使用本地 kubeconfig 中的身份访问远端集群，无需为 cereals 在集群内额外部署服务。

当前源码版本为 **1.0.0**，界面与文档以中文为主，采用 **Apache-2.0** 许可证。远端集群的网络连通性和操作权限由你准备，应用通过 Kubernetes API 执行查询与操作。

[下载发行版](https://github.com/cereals-af/cereals-desktop/releases) · [使用指南](docs/user-guide.md) · [本地开发与构建](docs/development.md) · [问题反馈](https://github.com/cereals-af/cereals-desktop/issues)

## 使用场景

如果你已有一个或多个远端 Kubernetes 集群，希望在自己的电脑上查看资源、修改配置、排查 Pod 和管理工作负载，可以通过 cereals 导入 kubeconfig 后操作。你的电脑需要能够访问目标 API Server；使用内网地址的集群通常还需要对应的 VPN 或网络连接。

应用沿用 kubeconfig 对应身份的权限，不会因为安装桌面客户端就获得额外的集群权限。操作前请确认当前集群、命名空间和资源；修改资源时检查服务端预检结果与差异，再确认提交。

## 主要功能

| 能力               | 说明                                                                                                  |
| ------------------ | ----------------------------------------------------------------------------------------------------- |
| 多集群连接         | 导入多个 kubeconfig 和 Context，设置默认集群，独立连接与断开                                          |
| 资源浏览           | 按命名空间和标签筛选、分页查询、按需实时更新，浏览内置资源与自定义资源                                |
| 工作负载管理       | Deployment、StatefulSet、DaemonSet、ReplicaSet、Job、CronJob 的专用页面及适用的扩缩容、发布与回滚操作 |
| 可视化与 YAML 编辑 | 常用资源表单、YAML 编辑、Schema 提示、服务端预检、差异确认和并发冲突提示                              |
| 网络与配置         | Service、Ingress、NetworkPolicy、ConfigMap 和 Secret 管理                                             |
| 节点与存储         | 节点详情、封锁与排空，PVC、PV、StorageClass 浏览及适用的 PVC 扩容                                     |
| Pod 排障           | 日志跟随与导出、交互终端、单文件传输、TCP 端口转发、多会话操作                                        |
| 资源用量           | 通过集群中的 Metrics Server 查看 Pod 和 Node 的当前 CPU、内存用量                                     |
| 桌面集成           | 托盘、关闭窗口行为设置、单实例唤回和应用内更新                                                        |

具体操作与限制见 [使用指南](docs/user-guide.md)。可用功能取决于集群提供的 API、当前身份的 RBAC 权限及容器内可用工具。

## 下载与安装

在 [GitHub Releases](https://github.com/cereals-af/cereals-desktop/releases) 选择与你的平台匹配的安装包。若尚无可下载版本，可按下方说明从源码构建。

| 平台                              | 1.0.0 安装包                            | 安装方式                                            |
| --------------------------------- | --------------------------------------- | --------------------------------------------------- |
| macOS，Apple Silicon，15.0+       | `cereals-1.0.0-macos-arm64.dmg`         | 打开 DMG，将 `cereals.app` 拖入 Applications 后运行 |
| Windows 10/11，Intel / AMD x86-64 | `cereals-1.0.0-windows-amd64-setup.exe` | 运行安装向导；需要 WebView2 Runtime                 |

发行附件提供 SHA-256 校验文件。macOS 当前打包使用本地 ad-hoc 签名，尚未配置 Developer ID 签名和 Apple 公证；Windows 尚未配置 Authenticode 签名，系统可能提示发布者未验证。Windows 实机安装和跨版本更新仍需验证，Linux 暂无已验证的发行包。

## 快速开始

1. 准备目标集群的 kubeconfig，并确认本机能访问其中的 API Server 地址。
2. 启动 cereals，在「集群连接」导入配置文件，或读取本机 `~/.kube/config`。
3. 选择 Context 并连接。如果 kubeconfig 使用外部认证插件，先确认插件命令及来源。
4. 选择集群、命名空间与资源，查看状态、日志或打开终端。
5. 修改资源时先运行服务端预检，检查差异，再确认提交到集群。

应用保存 kubeconfig 文件路径和界面设置，不复制 kubeconfig 凭据。新安装默认使用系统用户配置目录下的 `cereals`；已有旧版配置的兼容方式见 [配置与数据](docs/user-guide.md#配置与数据)。

关闭主窗口可以保留托盘和后台会话；从托盘选择「退出 cereals」才会结束应用。macOS 也可使用 `⌘Q` 退出。

## 从源码运行

准备 Go 1.26 或更高的兼容版本、Node.js 22.15+、npm，以及对应平台的 Wails 原生构建依赖。macOS 需要 Command Line Tools。Wails CLI 与运行时均固定为 **v3.0.0**。

```sh
git clone https://github.com/cereals-af/cereals-desktop.git
cd cereals-desktop

go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
go mod download
npm --prefix frontend ci

# 启动原生桌面开发窗口与前端热更新
wails3 task dev
```

确保 Go 的可执行文件目录已加入 `PATH`。如需生产构建，在仓库根目录运行：

```sh
# 生成绑定、构建前端和当前平台的桌面程序
wails3 task build

# 在 macOS 上生成本地应用包
wails3 task package:macos
```

开发构建不启用自动更新；macOS 裸可执行文件也不启用自动更新，需要从 `cereals.app` 启动。完整的依赖、测试、打包和代码目录说明见 [本地开发与构建](docs/development.md)。

## 自动更新

发行构建启动 30 秒后检查更新，此后每 6 小时检查一次；也可使用侧栏、托盘或 macOS 应用菜单中的检查更新入口。更新源为本仓库的正式 GitHub Release。

发现新版本后，应用下载匹配当前平台和架构的更新附件并进行 SHA-256 校验。点击更新窗口中的「重启并应用」（`Restart & Apply`）才会替换程序并重启。请先保存编辑草稿并结束需要保留的终端、日志和端口转发会话。

1.0.0 是更新基线，只有发布更高版本后才会提示升级。构建和上传更新附件的方法见 [更新与发布](docs/updates.md)。

## 文档

- [使用指南](docs/user-guide.md)：连接远端集群、资源操作、Pod 排障、配置与常见问题。
- [本地开发与构建](docs/development.md)：环境准备、代码结构、开发运行、检查测试和安装包构建。
- [更新与发布](docs/updates.md)：更新机制、附件格式、版本配置和发行步骤。

## 反馈与贡献

欢迎通过 [Issues](https://github.com/cereals-af/cereals-desktop/issues) 提交问题或建议，通过 [Pull Requests](https://github.com/cereals-af/cereals-desktop/pulls) 贡献代码和文档。

反馈问题时，请提供应用版本、操作系统及架构、Kubernetes 版本、复现步骤、预期行为和实际结果。日志与截图请先脱敏，不要公开 kubeconfig、Token、私钥、Secret 内容或其他集群敏感信息。

贡献前请阅读 [提交约定](docs/development.md#提交约定)，围绕一个明确目的提交变更，运行与影响范围匹配的检查，并在说明中列出验证结果和未验证的部分。功能实施计划和开发过程记录仅在本地保留，不放入公开文档。

## 许可证

本项目采用 [Apache License 2.0](LICENSE)，版权声明见 [NOTICE](NOTICE)。第三方依赖遵循各自的许可证。
