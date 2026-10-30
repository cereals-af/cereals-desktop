# cereals 图标素材

- `appicon.png`：2026-10-28 按用户要求采用的透明背景版图标，1254 × 1254 RGBA。保留绿色几何立方体和深青色圆角底板，圆角外留白使用真实透明通道，并清理底板外沿的杂色及发光。与 `previews/appicon-user-transparent.png` 一致，详见 [透明版本及编辑提示词](previews/appicon-user-transparent.md)。
- `tray-template.png`：44 × 44 黑色透明模板图标，供 macOS 系统着色。
- `tray.png` / `tray-dark.png`：绿色和浅绿色托盘图标，供其他桌面平台使用。
- 托盘图标由 `tools/trayicons` 的立方体线段源生成，以保证小尺寸可读性；没有缩小复杂的桌面原稿来充当托盘图标。
- 运行 `wails3 task icons` 可重新生成托盘 PNG、`build/darwin/icon.icns` 和 `build/windows/icon.ico`。生成产物随源码提交，正常打包无需调用图像服务。

当前版本基于用户图片使用内置 imagegen 处理透明背景并清理边缘，未使用 API/CLI 回退。生成后的素材保存在项目内，正常构建不依赖图像生成服务。[原图试用记录](previews/appicon-user.md)、[上一版品牌浅绿图标及提示词](previews/appicon-mint.md)和[中间版本记录](previews/README.md)随素材保留，可用于对比或恢复。

以下为最初高光版的生成提示词，仅用于保留设计过程：

```text
Use case: logo-brand. Asset type: production macOS desktop app icon, square 1024x1024 PNG with true alpha transparency outside its rounded square. Primary request: create an elegant, original app icon for creals, a local Kubernetes cluster operations desktop client. Existing brand is a simple isometric cube outline, dark slate navy #23313e and emerald green #159c7a with mint #64d3b6. Center a single bold geometric isometric cube emblem, made of thick luminous mint and emerald edges with clean white-mint highlights, suggesting a neatly connected cluster. Put it on a softly rounded-square deep slate tile with very subtle dimensional bevel and controlled depth, premium native desktop feel. Tile occupies about 88% of square canvas, transparent margin outside. Straight-on icon, perfectly centered and balanced, strong simple silhouette readable at 32px. Use a minimal polished 3D-vector aesthetic, no extraneous nodes, no thin micro details, no text, no letters, no Kubernetes wheel, no watermark, no scene, no mockup, no multiple icon variants. Background outside the single tile must be genuinely transparent.
```
