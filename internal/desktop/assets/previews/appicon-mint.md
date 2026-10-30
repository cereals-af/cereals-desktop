# 品牌浅绿图标预览

文件：`appicon-mint.png`。2026-10-28 使用内置 image_gen，以 `appicon-satin.png` 为编辑输入生成；未使用 API/CLI 回退。

以页面品牌图标的 `#64D3B6` 为主体色参考，提亮绿色中间调，减轻阴影，保留圆角轮廓、比例与柔和立体层次，避免白色强高光和发光。生成图包含渐变与抗锯齿，不代表每个绿色像素都等于指定色值。

2026-10-28 用户曾确认采用此版本，并同步生成 macOS ICNS 和 Windows ICO。随后按用户要求切换为[用户图片版本](appicon-user.md)进行试用；此处保留品牌浅绿版本及其编辑记录，可用于恢复。

完整提示词：

```text
Use case: precise-object-edit. Asset type: creals desktop app icon color revision. Edit target: the supplied satin cube icon. Change ONLY the green color palette and the strength of shading on the cube. Preserve the exact geometry, rounded edges, frame thickness, openings, proportions, camera angle, size, dark slate tile, placement, and transparent outer margin. The user chose the app's existing brand mint #64D3B6 (RGB 100, 211, 182) as the MAIN visible cube color. Match this clean light mint green closely across the large front-facing and top-facing surfaces; it should be the dominant color, not just a thin highlight. Make the right face nearly as bright as the left, with only a small tone difference. Use only gentle mint variations: around #72D9BF for soft light areas and #50BFA3 for shaded areas, modestly deeper green only within narrow recessed inner faces. The previous version looks much too dark and murky. Lift the cube's midtones substantially while keeping a smooth restrained satin material. Retain refined rounded geometry and soft dimensional modeling, but reduce heavy directional shadows. No white highlights, no glow, no neon, no bloom, no shiny plastic reflections, no washed-out nearly-white mint, no rough or grainy texture, no polygonal redesign. Keep the dark navy rounded-square base exactly as it is. Single finished icon, no text, no other elements, true alpha transparency outside the tile.
```
