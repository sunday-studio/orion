import { defineConfig } from "astro/config";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://orion.sunday.studio",
  vite: { plugins: [tailwindcss()] },
  markdown: { shikiConfig: { themes: { light: "github-light", dark: "github-dark" } } },
});
