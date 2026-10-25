# Orion Mobile design

Published on the design canvas (https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY), pages **Mobile IA**, **Mobile flows** and **Mobile screens**. Product plan: `docs/plans/mobile-app-proposal.md`.

`build/` generates the artboards (`mobile-*.dc.html`, light and dark) and merges them into the canvas `project/canvas.json`:

```sh
python3 docs/design/mobile/build/build.py <canvas.json> <out-dir>
```

Built from the real Orion components (tracker, expected vs actual, one green `#1f7a50`) with Linear-mobile patterns: inbox-style flat rows with status ring icons, swipe actions, property pills with picker sheets, a pinned note bar and a flat tab bar. Values come from the canvas artboards, not `design-language.md` v2.
