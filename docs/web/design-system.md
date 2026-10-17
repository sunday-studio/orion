# Console design system

Location: `apps/console/src/design-system/`. Built on Base UI (`@base-ui/react`), Tailwind v4 and
`class-variance-authority`. Spec: `docs/design/design-language.md`. Live reference: sign in and open
`/design-system` (tokens, every component, theme switcher).

Import everything from the barrel: `import { Button, DataTable } from "@/design-system"`.

## Tokens

`tokens.css` defines colours in `@theme` (so Tailwind generates `bg-card`, `text-ink`,
`border-hairline`, `text-quiet`, `bg-brand`, `bg-destructive`, `bg-dot-up` ...) and shadows
(`shadow-card`, `shadow-overlay`). Dark mode overrides the same variables under
`:root[data-theme="dark"]`; call `applyTheme("light" | "dark" | "system")` (stored per browser).

Typeface: `--font-sans` is `"TWK Lausanne", system-ui, ...`. The licensed files are not in the repo
(trial builds in `fonts/` must never be committed). To ship the licence, add `@font-face` rules for
TWK Lausanne 300 and 500 and optionally set `--font-weight-body: 300`.

The legacy shadcn-style `components/ui` folder and its `bg-accent`/`bg-muted` style tokens are gone.
Every screen imports from `@/design-system`; keep token names descriptive (`brand`, `quiet`, `hairline`).

## Components

| Component | Notes |
|---|---|
| `Button` | variants `primary secondary destructive ghost link`; sizes `sm md icon icon-sm`. `buttonStyles()` styles trigger elements. |
| `Badge`, `StatusBadge` | tones: up down degraded unknown maintenance stale covered resolved info critical medium. label plus glyph shape (dot, square, half, hollow, pause), so status is never colour-only. `StatusBadge status="down"` maps API strings to a tone. |
| `Input`, `Checkbox`, `Switch` | pass `aria-label` when there is no visible label. |
| `Select` | typed `options`, optional `label` prefix. `Combobox` is the searchable variant. |
| `Tabs`, `TabsList`, `TabsTab`, `TabsPanel` | centered; arrow keys activate. |
| `Dialog*`, `ConfirmDialog` | `ConfirmDialog` takes a title, a description naming the blast radius, and a confirm label. |
| `Drawer*` | right-side detail panel, Escape closes. |
| `Popover*`, `Tooltip`, `Menu*` | Tooltip wraps one focusable child and also shows on focus. |
| `ToastProvider`, `useToast().add({ title, description })` | mount the provider once; the close button appears when the toast is hovered or focused (Base UI behaviour). |
| `Card*`, `Field` | `Field` is the pale container for groups of cards. |
| `Textarea`, `Breadcrumb*`, `DetailCard`, `DetailGroup`, `ListPagination`, `PageHeader`, `DataTableLink` | shared layout and detail pieces moved here from `components/shared`. |
| `EmptyState`, `Skeleton` | empty and error states end in a button. |
| `DataTable`, `FilterBar`, `Table*` | see below. |

## Data table pattern

Follows the OpenStatus rules in AGENTS.md: TanStack Table with schema-driven columns, `nuqs` URL
state, server-side pagination and filtering when the API supports it.

```tsx
const fields: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search" },
  { type: "select", key: "status", label: "Status", options },
];
const table = useDataTableUrlState({ fields, limit: 40 });      // q, status, page, sort in the URL
const params = { q: filterParam(fields[0], table.values.q), offset: table.offset };
<FilterBar fields={fields} values={table.values} onChange={table.setFilter} onClear={table.clearFilters} />
<DataTable columns={columns} data={rows} isLoading={q.isLoading}
  error={q.error ? { message: "Unable to load", onRetry: q.refetch } : undefined}
  onResetFilters={table.hasFilters ? table.clearFilters : undefined} />
```

`fields` must be a module-level constant. Sorting (`sort`, `onSortChange`) only reorders the visible
page, so skip it on server-paginated tables. Rows are keyboard operable when `onRowClick` is set.

## Rules for new UI

- Build from these components; put new shared controls in `design-system/`, not in feature folders.
- Every control has a visible focus ring (`focusRing` in `styles.ts`) and a keyboard path.
- Destructive actions use `ConfirmDialog` and name what will be lost.
- Files stay under 500 lines; names are kebab-case.

## Tests

`apps/console/e2e/design-system.spec.ts` drives the preview page: dialog focus return and Escape,
confirm button visibility, tab arrows, checkbox/switch Space, select, combobox filtering, menu,
tooltip on focus, drawer, popover, toast, data table URL state and row Enter, dark theme.
