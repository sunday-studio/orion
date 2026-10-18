import { defineConfig } from "astro/config";
import mdx from "@astrojs/mdx";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://orion.sunday.studio",
  integrations: [mdx()],
  vite: { plugins: [tailwindcss()] },
  markdown: { shikiConfig: { theme: "github-dark", wrap: false } },
});
