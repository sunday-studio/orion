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
- `src/layouts/base.astro`: page shell, Open Graph and Twitter tags, canonical URL. Pass `ogImage` to use another share image set.
- `src/pages/search.json.ts`: search index built from the docs collection at compile time.
- `src/scripts/`: the only client JavaScript: ⌘K search, copy buttons, table-of-contents highlight and saved Quickstart progress.
- `src/styles/`: `tokens.css` (design tokens), `base*.css`, `home.css`, `docs.css`, `search.css`, `responsive.css`.
- `public/shots/`: Console screens used by the home page tour.

## Share images

Every page has Open Graph and Twitter card images in three sizes, in `public/og/`:

| Size | Used by |
|---|---|
| `1200x630` | Facebook, LinkedIn, Slack, Discord and most link previews |
| `1200x600` | X / Twitter (`twitter:image`) |
| `1200x1200` | square cards such as WhatsApp and Telegram (a second `og:image`) |

There are three layouts: the home page, the docs home, and a title-led layout generated for every docs article from its frontmatter. `src/layouts/base.astro` writes the tags; pass `ogImage="/og/<name>"` to point a page at another set.

Regenerate after changing copy or adding a docs page. It needs Chrome or Chromium and nothing else:

```sh
node scripts/og/generate.mjs
# CHROME=/path/to/chrome            use a different browser binary
# ORION_OG_FONT_CSS=/path/font.css  render with the licensed typeface
```

The committed images were rendered with the licensed typeface. Without it the script falls back to the system sans, so text looks slightly different.

## Adding a docs page

Create `src/content/docs/<slug>.mdx` with `title`, `description`, `group` (Start, Guides or Reference) and `order`. Headings at level 2 become the on-this-page list and are searchable.

## Design source

Visual design follows `docs/design/design-language.md`. The approved layouts for the home page and docs are on the Orion Console Design canvas (Website and Docs pages); see `docs/design/website-handoff.md`.

TWK Lausanne is a licensed font and is not committed. The site falls back to the system sans stack until a licensed copy is added as a local `@font-face`.
