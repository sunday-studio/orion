/// <reference types="vitest/config" />
import {defineConfig} from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import tsconfigPaths from "vite-tsconfig-paths";
import {playwright} from "@vitest/browser-playwright";
import svgrPlugin from "vite-plugin-svgr";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), tsconfigPaths(), svgrPlugin()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./setup-test.ts"],
    coverage: {
      provider: "v8",
    },
    browser: {
      provider: playwright(),
      instances: [{browser: "chromium"}],
    },
  },
});
