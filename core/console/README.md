# Palm Take-Home

A small React application built with Vite that allows users to browse, search, and analyze GIF interactions.

## Project Highlights

- **Random GIF on Load**  
  Displays a random animated GIF when the home page loads.

- **Live GIF Search**  
  Search GIPHY with instant results. The random GIF view is hidden while searching, and results are displayed in a responsive grid.

- **GIF Detail Pages**  
  Click any GIF to view details including title, rating, and a link to the original source.

- **Analytics**  
  Tracks user interactions with GIFs in a session-persistent, sortable, and paginated table, complemented by lightweight charts.

- **Tech Stack**  
  React 18, Vite, Tailwind CSS, TanStack Query, TanStack Table, Recharts, and Playwright.

- **Code Quality**  
  ESLint is configured to enforce consistent style and best practices.

## Prerequisites

- You need to have `bun` installed
- Create a `.env` file with `VITE_GIPHY_KEY` as the only secret

## Development

```bash
bun install
bun dev
```

## Scripts

- `bun run dev` — start the development server
- `bun run build` — build for production
- `bun run preview` — preview the production build
- `bun run lint` — run ESLint
- `bun run test:unit` — run unit and integration tests (Vitest)
- `bun run e2e` — run end-to-end tests (Playwright)
- `bun run generate:api` — generate TypeScript API SDK from OpenAPI spec

## Architecture

### Folder Structure

```
src/
├── assets       # SVG icons and static assets
├── components   # Shared UI components
├── context      # Analytics context provider
├── features     # Page-level views
├── hooks        # Data-fetching and domain hooks
├── tests        # Unit and integration tests
└── utils        # Helpers and API utilities
```

### Architectural Principles

- **URL-driven state**  
  State that can be represented in the URL (for example search terms or active views) is kept in the URL where appropriate.

- **Server state via React Query**  
  React Query prevents duplicate requests, handles caching, and manages loading and error states.

- **Consistent naming**  
  File and folder names use lowercase kebab-case to improve discoverability and avoid case-sensitivity issues across environments.

- **SVG handling**  
  SVGs are inlined using `vite-plugin-svgr` for better performance and flexibility compared to image assets.

## Third-Party Libraries & Rationale

**Vite**  
Fast development server, minimal configuration, and excellent TypeScript support.

**React Router**  
Declarative routing with nested layouts, strong community adoption, and clear documentation.

**TanStack Query**  
Simplifies async data fetching with caching, request deduplication, refetching, and strong TypeScript support.

**TanStack React Table**  
Headless table logic with built-in support for sorting and pagination.

**Recharts**  
Simple, declarative API for responsive charts and lightweight analytics visualization.

**Ky**  
Lightweight alternative to Axios with a cleaner API and sensible defaults.

**Tailwind CSS**  
Enables rapid UI development and easy prototyping without managing large CSS files.

**Vitest**  
Fast, Vite-native test runner with minimal configuration.

**Playwright**  
Preferred choice for end-to-end testing due to reliability, strong TypeScript support, and ease of setup.
