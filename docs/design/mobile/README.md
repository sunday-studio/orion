# Orion Mobile design

Published on the design canvas (https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY), pages **Mobile IA**, **Mobile flows** and **Mobile screens**. Product plan: `docs/plans/mobile-app-proposal.md`.

`build/` generates the artboards (`mobile-*.dc.html`, light and dark) and merges them into the canvas `project/canvas.json`:

```sh
python3 docs/design/mobile/build/build.py <canvas.json> <out-dir>
```

Built from the real Orion components: pale field holding white cards, lifecycle tracker, expected vs actual cards, pale-header boxes with uptime bars, segmented-tray nav, one green `#1f7a50`. Values come from the canvas artboards, not `design-language.md` v2.
