# 用户图片透明背景版

2026-10-28，按用户要求将原图试用版替换为透明背景版。当前文件为 `appicon-user-transparent.png`，同步复制到 `../appicon.png`，并通过 Wails 生成 macOS ICNS 和 Windows ICO。

使用内置 imagegen 编辑，未使用 API/CLI 回退。编辑输入为基于用户图片生成的第一版透明试稿。保留绿色几何立方体与深青色底板，清理外沿杂色并去除发光边缘，圆角外部保留真正的 alpha 透明通道，并保留底板与立方体内部的深色填充。

最终编辑提示词：

```text
Use case: background-extraction. Asset type: production desktop application icon. Edit the supplied transparent icon, preserving the cube and all interior colors and geometry. Replace only the damaged outermost rim of the dark teal rounded-square tile with a perfectly clean smooth rounded-square silhouette. Remove ALL lime-green and cyan fringe, isolated particles, jagged pixels, stray marks, exterior glow and exterior shadows. The boundary may be inset very slightly to erase the contaminated outermost pixels. Use a crisp dark-teal edge with normal subtle antialiasing, no bright outline. Keep the same overall tile shape, corner curvature, center, scale and padding. Keep the entire dark tile and cube opaque, including the cube openings. Beyond that single clean rounded-square boundary, every pixel must be truly alpha-transparent. Preserve the existing flat geometric green cube precisely; do not change its colors, dimensions, position or lighting. No black rectangle, no solid exterior background, no checkerboard texture, no extra elements, no text. Deliver a single square icon with transparent outer margin.
```
