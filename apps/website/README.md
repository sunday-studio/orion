# Orion website

The public site and documentation, built with Astro 5. Static output, no runtime server.

```sh
cd apps/website
npm install
npm run dev      # http://localhost:4321
npm run build    # writes dist/
```

## Layout

- `src/pages/index.astro`: the home page. Sections live in `src/components/home/`.
- `src/pages/docs/`: docs home and the article route. Content is MDX in `src/content/docs/`.
- `src/components/docs/`: MDX components (`callout`, `code-tabs`, `steps`, `done`, `cmd`, `badge`).
- `src/layouts/base.astro`: page shell, Open Graph and Twitter tags, canonical URL. Pass `image` to use another share image.
- `src/pages/search.json.ts`: search index built from the docs collection at compile time.
- `src/scripts/`: the only client JavaScript: ⌘K search, copy buttons, table-of-contents highlight and saved Quickstart progress.
- `src/styles/`: `tokens.css` (design tokens), `base*.css`, `home.css`, `docs.css`, `search.css`, `responsive.css`.
- `public/shots/`: Console screens used by the home page tour.

## Share images

`public/og.png` and `public/og-docs.png` are the 1200 × 630 Open Graph images. Their sources are `og/home.html` and `og/docs.html`. To re-render after changing the copy:

```sh
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless --hide-scrollbars \
  --window-size=1200,630 --screenshot=public/og.png file://$PWD/og/home.html
```

The committed images were rendered with the licensed typeface. Without it the template falls back to the system sans, so the text will look slightly different.

## Adding a docs page

Create `src/content/docs/<slug>.mdx` with `title`, `description`, `group` (Start, Guides or Reference) and `order`. Headings at level 2 become the on-this-page list and are searchable.

## Design source

Visual design follows `docs/design/design-language.md`. The approved layouts for the home page and docs are on the Orion Console Design canvas (Website and Docs pages); see `docs/design/website-handoff.md`.

TWK Lausanne is a licensed font and is not committed. The site falls back to the system sans stack until a licensed copy is added as a local `@font-face`.
