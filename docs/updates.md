# cereals 更新与发布

> **这是一个 vibecoding 项目，主要通过 AI 辅助编码构建。介意勿用。**
>
> **本项目面向在本地电脑通过 kubeconfig 连接远端 Kubernetes 集群的运维人员、开发者和集群管理员。**

本文面向维护更新附件和发行版本的贡献者。本地编译及打包环境见 [本地开发与构建](development.md)，用户升级方法见 [使用指南](user-guide.md)。

应用直接使用 Wails 3.0.0 的 `app.Updater` 和 GitHub provider。更新源为 `cereals-af/cereals-desktop` 的最新正式 Release，预发布版本不会推送给稳定版用户。版本比较支持 Git 标签的 `v` 前缀，相同和更低版本不更新。

## 客户端行为

- 发行构建启动 30 秒后检查，此后每 6 小时检查；没有更新时不弹窗，后台网络错误仅记录错误。
- 侧栏、托盘、macOS 应用菜单提供手动检查。检查/下载串行，退出应用会取消请求并等待后台任务结束。已下载待重启时跳过自动检查。
- 有更新时使用 Wails 自带窗口展示说明、下载、校验并暂存；窗口按钮采用 Wails 默认英文。用户点击 **Restart & Apply** 后才通过框架 helper 替换程序并重启，重启前应保存草稿、结束运维会话。
- helper 模式在配置及集群初始化之前处理。开发构建禁止更新；macOS 仅在标准 `.app/Contents/MacOS/` 结构中启用更新，防止用完整应用包替换裸可执行文件。Linux 暂不启用更新。
- SHA-256 校验为必需项；缺少 `SHA256SUMS`、缺少匹配条目、校验值长度错误或下载内容不匹配时拒绝更新。校验值来自同一 GitHub Release，它提供完整性验证，当前未配置独立 Ed25519 发布签名。

## 发布附件

`wails3 task package:installers` 构建安装器及专用更新附件，最后生成更新器所需的 `bin/updates/SHA256SUMS`。

| 用途             | 1.0.0 附件示例                           | 位置              |
| ---------------- | ---------------------------------------- | ----------------- |
| macOS 首次安装   | `cereals-1.0.0-macos-arm64.dmg`          | `bin/installers/` |
| Windows 首次安装 | `cereals-1.0.0-windows-amd64-setup.exe`  | `bin/installers/` |
| macOS 自动更新   | `cereals-1.0.0-update-darwin-arm64.zip`  | `bin/updates/`    |
| Windows 自动更新 | `cereals-1.0.0-update-windows-amd64.exe` | `bin/updates/`    |
| 更新完整性校验   | `SHA256SUMS`                             | `bin/updates/`    |

macOS ZIP 内是完整、已签名的 `cereals.app`，更新时连同 Info.plist、资源和签名一起替换。Windows 更新附件是独立程序，不能用 NSIS 安装器替代。附件匹配明确排除 DMG、setup.exe 和校验侧文件。macOS 架构随构建宿主；Windows 当前脚本构建 amd64。尚未发布对应架构附件会报告无匹配文件，不能回退到其他平台。

1. 更新 `frontend/package.json` 与锁文件中的版本，以及 `build/config.yml`、Windows manifest 中的发行版本。侧栏、Go 更新器、User-Agent、安装器和 macOS Bundle 通过构建脚本使用 npm 版本。
2. 为本次发行准备干净的 `bin/updates/`。校验工具会拒绝混入其他版本、未知文件、目录和符号链接。
3. 运行 `wails3 task package:installers`。跨机器收集其他架构附件时，将它们合并到同一个更新目录，再运行 `wails3 task updates:checksums`。
4. 创建对应标签的草稿 Release，上传该版本的安装器、全部专用更新附件和 **名称严格为 `SHA256SUMS`** 的校验文件。逐项核对版本和校验值，再公开 Release。不要先公开版本再补附件。
5. 从上一版本的已安装应用执行检查、下载、重启，确认新版本、配置和菜单正常。macOS 必须从可写的安装位置运行，不能从只读 DMG 内执行更新；Windows 在当前用户安装目录验证。未接入更新器的旧 creals 版本需要先手工安装 cereals 1.0.0。

本地构建和打包命令不会创建 Git 标签、上传附件或公开 Release，这些发行操作需要单独执行。1.0.0 是更新基线，不会提示更新到同一版本。

## 安装兼容与分发要求

显示名称、可执行文件、应用包和安装器改为 cereals。系统标识 `cn.creals.desktop`、Kubernetes 字段管理器 `creals` 和既有表格偏好键保留。配置优先级为 `CEREALS_CONFIG_DIR`、旧 `CREALS_CONFIG_DIR`、已有 `cereals` 目录、已有 `creals` 目录，最后为新建的 `cereals` 目录；不搬移或删除用户配置。

Windows 安装器沿用已有安装位置并清理旧程序和快捷方式；旧程序仍在运行时会停止安装。macOS 手工迁移后可自行移除旧 `creals.app`，避免从旧入口启动。

macOS 当前仍为本地 ad-hoc 签名，Windows 未配置 Authenticode；正式跨机器分发需在对应平台检查签名、系统提示和安装路径权限。自动化测试不等价于真实操作系统的 helper 替换、重启、签名与回滚验收。

参考：[Wails 自动更新教程](https://v3.wails.io/tutorials/04-self-update-a-wails-app/)。实现以项目锁定的 v3.0.0 源码为准。
