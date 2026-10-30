# 桌面图标调整预览

`appicon-satin.png` 是 2026-10-28 使用内置 image_gen 编辑原版桌面图标得到的柔和高光预览，1254 × 1254 RGBA，保留透明边缘。保留原版圆角、比例和柔和层次，降低白色高光、发光和反射。

此图为未采用的中间版本。此前使用基于此图提亮得到的 `appicon-mint.png`，详见 [品牌浅绿版本记录](appicon-mint.md)。当前按用户要求采用[用户图片的透明背景版](appicon-user-transparent.md)。此前扁平化方案因视觉反馈不采用。

使用内置工具模式，未使用 API/CLI 回退。编辑输入为原版 `../appicon.png`，完整提示词：

```text
Use case: style-transfer. Edit target: this original creals desktop icon. This is a precise MATERIAL AND LIGHTING RETOUCH, not a redraw. Preserve EXACT original rounded cube silhouette, softly radiused edges, all proportions, cube size, openings, camera angle, tile size, dark navy background, composition, and alpha transparency. Keep the original elegant smoothly rounded craftsmanship and gentle dimensional depth. The only problem to fix: excessive bright white highlights and neon glow. Turn down those effects. Replace the almost-white top-facing cube surfaces with a restrained medium seafoam jade green (around #439F8B), keep darker emerald green sides (around #167C68). All colored faces should harmonize with these restrained tones; nothing fluorescent, no white strips or luminous rims. Use a sophisticated smooth SATIN finish with very subtle soft shading, not chalky, not grainy, not rough, not hard flat polygonal art. The dark slate tile should retain its gently rounded outline and subtle softness but remove the bright blue reflective rim and heavy glossy bevel. Reduce contrast of tiny highlights drastically. Keep enough soft dimensional modeling that the shape stays refined and clearly legible, but there must be no dramatic 3D lighting, glow, bloom, metallic shine or plastic reflection. This should look exactly like the original beautiful icon photographed under soft subdued diffuse light, not rebuilt as a different angular simplified cube. True transparent margin outside the tile. No text or additional elements.
```
