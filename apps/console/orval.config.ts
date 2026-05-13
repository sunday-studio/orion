import { defineConfig } from "orval";

export default defineConfig({
  orion: {
    input: "../core/openapi.yaml",
    output: {
      target: "./src/orion-sdk/index.ts",
      client: "react-query",
      httpClient: "fetch",
      baseUrl: { getBaseUrlFromSpecification: false, baseUrl: "" },
      override: {
        mutator: {
          path: "./src/lib/custom-instance.ts",
          name: "customInstance",
        },
        query: {
          useQuery: true,
        },
      },
    },
  },
});
