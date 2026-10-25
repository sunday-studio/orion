import { defineConfig } from "astro/config";
import mdx from "@astrojs/mdx";
import sitemap from "@astrojs/sitemap";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://orion.thesunday.studio",
  integrations: [mdx(), sitemap({ filter: (page) => !page.endsWith("/404/") })],
  vite: { plugins: [tailwindcss()] },
  markdown: { shikiConfig: { theme: "github-dark", wrap: false } },
});
