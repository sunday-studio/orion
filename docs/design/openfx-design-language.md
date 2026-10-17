> Reference only (2026-10-17). OpenFX informed the container, number and stat patterns; the shipped spec is `design-language.md`, and some OpenFX choices (pill buttons, dithered art on product screens) were not adopted.

# OpenFX design language, and how Orion can use it

Source: https://www.openfx.com, surveyed 2026-10-16. The sitemap lists 145 English URLs that collapse into about 28 page templates. I opened every template in the built-in browser, read its text and measured computed styles (fonts, sizes, colors, radii, section heights). Not inspected: hover and scroll animation, the Resources dropdown contents, the footer, the Portuguese (pt-BR) pages, and blog bodies that load lazily. I did not interact with the cookie banner.

## 1. What each page type does

| Template | Pages | Structure | Notable device |
|----------|-------|-----------|----------------|
| Home | `/` | Dark-green announcement strip, hero ("Move money faster"), investor logo row, dotted world map with flag pins, stat row, feature panels, CTA | Dithered world map as the hero image |
| Product | `/product` | Hero, settlement-speed stacked bar charts (Fiat, Stablecoin), 40+/25+/$20B+/24/7 stat row, five pale feature panels each with a mini product UI, 3-step onboarding, API example, solution cards, FAQ | Each panel = short title, gray one-line promise, one simplified UI mock |
| Solutions (6 segments + index) | `/solutions/*` | Hero with dithered pixel icon, giant number ("$10,000,000" 56px), "Traditional vs OpenFX" two-card compare (gray vs dark green), five benefit headings, corridors, API tabs (Quote, Trade, Settle) with REST/FIX/Streaming, stat | Before/after comparison with one huge number |
| Currencies | `/currencies` | Hero, dotted world map with flag pins, currency pair browser | Map as the interface |
| Get quote | `/get-quote` | Hero then a live rate widget (loads late) | Tool embedded in a marketing page |
| Liquidity | `/liquidity` | Hero, LED dot-matrix ticker illustration ("USD > MXN BUY 10,000,000.00"), stat row, "as of Dec '25" mono caption, JSON request/response, treasury section | Pixel-ticker hero art |
| Accounts | `/accounts` | Hero with waitlist CTA, "Traditional" vs "OpenFX" compare cards | Same compare card pattern |
| Impact | `/impact` | Hero with two CTAs, an illustrative-example card (mono uppercase labels, red vs green), testimonials | "ILLUSTRATIVE EXAMPLE" tag on hypothetical data |
| Company | `/company` | Green pixel-art skyline hero, mission, problem (five pain points), platform, values | Full-width illustrated banner |
| Careers | `/careers` | Short hero, accordion of departments with counts, role rows with location | Plain list, no cards |
| Ventures | `/ventures` | Hero, four-card "Where we invest", team, backers | Icon plus title plus paragraph cards |
| Events | `/events/*` | Gradient green hero with dithered event lettering, white pill CTA | Event poster in the brand art style |
| Reports | `/reports`, `/reports/*`, report landing pages | Hero over faint dotted map, report book cover on a soft stage, "REPORT • Q3 2026" mono eyebrow, Get Report CTA | Product shot of the PDF cover |
| Blog | `/blog`, `/blog/categories/*`, `/blog/*`, `/blog/authors/*` | 46px title, big cover card, text list; category filter chips; post with breadcrumb, title, author and share icons; author page with banner, avatar and bio | Cover art in mint on dark green with mono "OPENFX.COM / MOVE MONEY LIKE DATA" label |
| FAQ | `/faq` | Hero on a green dotted wash, five category cards with chevrons | Category cards, not a long accordion |
| Get a demo | `/get-a-demo` | Single column form: first/last name, work email, region as a grid of selectable tiles, product interest | Choice tiles instead of dropdowns |

## 2. Tokens (measured)

**Color**
- Ground white `#ffffff`, panel `#f9f9f9`, input `#f6f6f6` with `#e9e9e9` border.
- Text `#1a1a1a`; secondary `#676767`; tertiary `#545454`; dividers about `rgba(0,0,0,.05)`.
- Brand green `#2a9f69` for primary buttons, check marks and ticks; light green `#4cba83` for chart bars; deep green `#142a23` for the "OpenFX" half of comparisons and dark cards; mint text `#80ffaa` on dark.
- Chart secondaries: lavender `#c0bff7`, peach `#ffd29b`. Negative amounts red. The blog post "Why OpenFX chose green in a sea of blue" states the intent: one confident green against the usual fintech blue.

**Type**
- Display: Suisse Int'l, 35px page titles (-0.02em), 26 to 30px section titles at weight 400, giant numerals 56px/500.
- Body and UI: Inter 14 to 16px, 400/500, slightly negative tracking (-0.01em).
- Micro: IBM Plex Mono 12px, +0.1em tracking, uppercase for eyebrows and chart labels, used for code, JSON, axis ticks, "as of" dates, "REPORT • Q3 2026".
- Blog index title 46px; card titles 23px/600.

**Shape and space**
- Buttons and the Sign in chip are full pills (200px radius); cards 8 to 12px; inputs 3px; avatars round.
- Page panels sit on a 12px-radius `#f9f9f9` field with about 70px vertical padding; content column about 970px.
- Cards use a hairline shadow (`0 1px 2px rgba(0,0,0,.15)`), never heavy.
- Nav is a 60px white bar: logo left, Product, Company, Resources centered, Sign in right, hamburger.

**Motifs**
1. Dither and pixel imagery: dotted world map, LED ticker, pixel skyline, dithered Statue of Liberty, dotted event lettering.
2. One green.
3. Giant single number as the argument ("$10,000,000" pre-funding, "$5M → $50M").
4. Mock UI as illustration: tracking timeline, balance rows with flags, role avatars, step flow, API request beside its response.
5. Honest labelling: "illustrative example", "as of Dec '25", "Updated on Jul 26'".

## 3. Layout and interaction patterns

- **Section recipe**: pale panel, centered 26px title, one line of gray copy, one mock. Repeat. Generous air, one idea per panel.
- **Compare card pair**: left gray "Traditional" with red bullets, right deep-green "OpenFX" with check marks and a single hero number.
- **Stat strip**: four numbers separated by hairlines, mono captions.
- **Code tabs**: Quote | Trade | Settle on top, REST | FIX | Streaming below, dark code panel with a mono POST line and syntax-colored JSON.
- **Stepper**: "STEP 1" mono eyebrow, title, one sentence, CTA, mock on the other side, alternating sides.
- **Choice tiles**: forms present options as large tiles.
- **Filter chips** for blog categories; accordions with counts for careers; category cards for FAQ.
- **Banner**: dark-green strip with a "NEW" tag and a link above the nav for the latest report.

## 4. Voice

Plain, quantified, confident. Lead with a number or a verb ("Move money faster", "Under 10 mins"). Short sentences, no hedging, no exclamation marks. Every claim is tied to a figure and a date.

## 5. Applying it to Orion (square rule kept)

Orion's constraints win where they conflict: top navigation unchanged, no rounded corners. What transfers is the system around the shapes.

| OpenFX element | Orion adaptation |
|----------------|------------------|
| Single brand green, no blue | Keep indigo as the one accent in the Console, but move status "Up" to this green family; public status pages adopt `#2a9f69` as the page accent |
| Mono micro-labels in uppercase | Table headers, tile labels, eyebrows ("INCIDENT • OPEN") in a mono at 11 to 12px, +0.08em |
| Giant single number | Overview hero shows one large figure (open for 30m 12s, 99.94%) instead of paragraph text |
| Compare cards | Incident detail "Before / after", status page "Expected vs actual", monitor test "Last pass vs last fail" |
| Mock-UI-as-illustration | Empty states and onboarding show a faded example of the real table instead of a diamond glyph |
| Dither and pixel art | Replace the dotted diamond with a dotted map or LED ticker for the Overview and Servers empty states; uptime strips already read as a pixel texture |
| Stat strip with hairlines | Overview KPIs and Logs summary as one bordered strip, square cells, no gaps |
| "Illustrative" and "as of" labels | Every chart gets "as of 14:32:08 UTC" and the range in mono |
| Code tabs | Heartbeat setup (cron, curl, Python), API tokens, webhook payloads |
| Choice tiles | Add server OS picker, monitor type picker, channel type picker as large square tiles |
| Hairline shadow card | Already matches: 1px border, no shadow |
| Pill buttons | Not adopted. Buttons stay square |
| Pale `#f9f9f9` panels on white | Use as section grouping on Settings and Status page setup |
| Banner strip | A thin top strip for "Maintenance in 2 hours" or "Update available", above the header |

### Public status page in OpenFX style

Centered 970px column on white. 35px title, one gray line. Overall status as one giant statement with the green check ("All systems operational") or a single bold number of affected components. Components in a `#f9f9f9` panel, each with a name, a 90-day pixel strip and a mono uptime figure. Incident history as a flat list. Footer line "Updated 14:32 UTC" in mono. Subscribe uses choice tiles for components.

## 6. Open questions

- Should "Up" move from teal to this green across the Console, or stay teal in the Console and use green only on public pages?
- Should the Console adopt a mono for table headers, or keep the current sans?
- Do you want the dither and pixel motif in product empty states, or keep them plain?
