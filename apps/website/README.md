# Orion website

The public site and documentation, built with Astro 5. Static output, no runtime server.

```sh
cd apps/website
npm install
npm run dev      # http://localhost:4321
npm run build    # writes dist/
```

## Deploy to Cloudflare

The site is static. It deploys to Cloudflare Workers static assets, configured in `wrangler.jsonc` (assets from `dist/`, a real 404 page, trailing-slash handling). `public/_headers` sets security headers and long caching for the hashed `/_astro` files.

**One-time setup**

1. In Cloudflare, create an API token from the "Edit Cloudflare Workers" template and note your Account ID.
2. In the GitHub repository, add secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (Settings, Secrets and variables, Actions).
3. Run the **Website deploy** workflow once from the Actions tab. The first run creates the `orion-website` Worker and publishes it at `orion-website.<your-subdomain>.workers.dev`.
4. Attach the real domain in the dashboard (Workers & Pages, `orion-website`, Settings, Domains), or add the `routes` entry shown in `wrangler.jsonc`. The domain's zone must be on your Cloudflare account.
5. Set the real address in `astro.config.mjs` (`site`), `public/robots.txt` and the fallback in `src/layouts/base.astro`, then redeploy.

**Or let Cloudflare build from Git (no GitHub secrets needed)**

Cloudflare can watch the repository and build on every push. In the dashboard: Workers & Pages, Create, Import a repository, choose `sunday-studio/orion`, then set:

| Setting | Value |
|---|---|
| Worker name | `orion-website` (must match `name` in `wrangler.jsonc`) |
| Production branch | `main` |
| Root directory | `apps/website` |
| Build command | `npm run build` |
| Deploy command | `npx wrangler deploy` |
| Variable | `NODE_VERSION` = `22` |

Pushes to `main` deploy to production. Other branches build a preview version with its own URL if you leave non-production branch builds on (their command is `npx wrangler versions upload`). If you use this, you do not need the GitHub workflow below; leave `WEBSITE_DEPLOY_ENABLED` unset or delete `.github/workflows/website-deploy.yml`.

**Deploying**

- By hand from your machine: `npx wrangler login`, then `npm run cf:deploy`.
- By hand from GitHub: run the **Website deploy** workflow.
- Automatically on every push to `main` that touches `apps/website/`: set the repository variable `WEBSITE_DEPLOY_ENABLED` to `true`. It is off until you do. (Repository CI is switched off separately; see decision D-024.)
- Try the built site locally the way Cloudflare serves it: `npm run cf:preview`.

Wrangler usage metrics are disabled in the workflow (`WRANGLER_SEND_METRICS=false`). Set the same variable locally if you want them off.

## Layout

- `src/pages/index.astro`: the home page. Sections live in `src/components/home/`.
- `src/pages/docs/`: docs home and the article route. Content is MDX in `src/content/docs/`.
- `src/components/docs/`: MDX components (`callout`, `code-tabs`, `steps`, `done`, `cmd`, `badge`).
- `src/layouts/base.astro`: page shell, Open Graph and Twitter tags, canonical URL. Pass `ogImage` to use another share image set.
- `src/pages/search.json.ts`: search index built from the docs collection at compile time.
- `src/scripts/`: the only client JavaScript: ⌘K search, copy buttons, table-of-contents highlight and saved Quickstart progress.
- `src/styles/`: `tokens.css` (design tokens), `base*.css`, `home.css`, `docs.css`, `search.css`, `responsive.css`.
- `public/shots/`: Console screens used by the home page tour.
- `public/_headers`, `wrangler.jsonc`, `src/pages/404.astro`: Cloudflare headers, config and the not-found page.

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
