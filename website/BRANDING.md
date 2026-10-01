# Project logo

The current identity uses the user-selected rambow.cloud ram, cloud, and wordmark, with a Go Gopher added at the front-left of the cloud. It replaces the earlier G-and-lightning design.

## Assets and integration

- `docs/assets/logo.png`: the full horizontal logo, including the rambow.cloud wordmark.
- `docs/assets/logo-ram-gopher.png`: the compact ram/cloud/Gopher emblem used by the documentation header, mobile navigation, and favicon.
- Both assets are transparent PNGs, produced using the built-in image generation tool on 2026-09-30.
- The full logo edits the user-supplied brand artwork. The compact version removes the wordmark and reframes the emblem with the same tool.
- `theme.logo` and `theme.favicon` in `mkdocs.yml` reference the compact asset. Its new filename avoids retaining the previous favicon in browser caches.
- Zensical's default modern theme renders and sizes the logo; no project template or CSS override is used.

## Gopher attribution

The Go Gopher was created by Renee French and is licensed under [Creative Commons Attribution 4.0](https://creativecommons.org/licenses/by/4.0/), as documented by the [Go project](https://go.dev/brand) and [The Go Gopher](https://go.dev/blog/gopher).

Changes: an AI-generated adaptation recolors and poses the Gopher within the user-provided ram/cloud logo, then creates a compact variant for small placements. Attribution appears in the documentation footer and `THIRD_PARTY_NOTICES.md`. This is an independent project, not an official Go or AWS distribution. The project MIT license does not replace the Gopher's attribution terms.

## Full-logo edit prompt

Edit the provided image, which is the user's chosen rambow.cloud logo. Preserve the existing ram with curled navy-and-cyan horns, white face, blue cloud silhouette, and exact lowercase "rambow.cloud" wordmark, including their shape, color and typography. Keep this identity recognizable and unchanged except for adding one mascot.
Add a small friendly recognizable Go Gopher mascot peeking out over the FRONT-LEFT edge of the blue cloud, below and left of the ram's face. The mascot is the classic Go Gopher character: cyan blue rounded head/body, two small round ears, two large protruding round white eyes with dark pupils, a little oval muzzle, two square front teeth, and two small hands resting on the cloud. Give it clean navy outlines and minimal flat shading to match the existing logo's navy/cyan palette. The gopher is secondary, around 35 percent the ram emblem's height. Keep both its eyes and teeth legible. It must not cover the ram's face or horns, or overlap any wordmark letters.
Composition: one finished horizontal logo lockup, cropped closely around the emblem and wordmark with modest clear padding; keep all edges and letters fully visible. Preserve true alpha transparency, including around the new gopher. No background rectangle, no drop shadow, no new text, no AWS marks, no unrelated badges, no watermark. Crisp production logo artwork rather than a mockup or concept sheet.

## Compact-logo edit prompt

Edit this exact rambow.cloud logo to make its compact app icon.
Keep ONLY the complete emblem on the LEFT: the navy/cyan curled-horn ram with white face, the blue cloud, and the cyan Go Gopher peeking over the cloud's front-left edge. Preserve those shapes, face expressions, colors, and arrangement. Remove the entire rambow.cloud wordmark on the right. Do not add a G, lightning bolt, new letters, extra symbols, or a background badge.
Reframe into a square canvas with the combined ram/cloud/gopher emblem centered and filling approximately 90 percent of the square, with even narrow transparent padding. Enlarge the existing emblem, do not leave the old horizontal canvas's empty space. The mascot's white eyes and teeth should stay crisp at small sizes. Keep the ram as the main subject and the gopher secondary.
Output a clean transparent PNG-style logo icon, true alpha background and negative space, no opaque background, no new outlines, no shadow, no mockup.
