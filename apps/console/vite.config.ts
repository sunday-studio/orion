import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { URL, fileURLToPath } from "node:url";

const coreURL = () => {
  const apiBaseURL = process.env.VITE_API_BASE_URL ?? "http://127.0.0.1:8080/v1";
  return apiBaseURL.endsWith("/v1") ? apiBaseURL.slice(0, -3) : apiBaseURL;
};

// https://vite.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  plugins: [react(), tailwindcss()],
  build: {
    rollupOptions: {
      output: {
        // Split stable vendor code from app code so route chunks and the entry stay small and cacheable.
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          if (
            /node_modules\/(react|react-dom|react-router|react-router-dom|scheduler)\//.test(id)
          ) {
            return "vendor-react";
          }
          if (id.includes("@base-ui") || id.includes("floating-ui")) return "vendor-base-ui";
          if (id.includes("@tanstack")) return "vendor-tanstack";
          return undefined;
        },
      },
    },
  },
  server: {
    proxy: {
      "/status": {
        target: coreURL(),
        changeOrigin: true,
      },
    },
  },
});
