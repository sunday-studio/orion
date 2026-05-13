import { defineConfig } from "orval";

const HTTP_METHODS = ["get", "post", "put", "patch", "delete"] as const;

function unwrapSuccessEnvelope(spec: any): any {
  for (const path of Object.values(spec.paths ?? {})) {
    for (const method of HTTP_METHODS) {
      const operation = (path as Record<string, any>)[method];
      if (!operation?.responses) continue;

      for (const [status, response] of Object.entries(operation.responses) as [string, any][]) {
        if (!status.startsWith("2")) continue;

        const schema = unwrapEnvelopeSchema(response?.schema);
        if (schema) response.schema = schema;

        const content = response?.content?.["application/json"];
        const contentSchema = unwrapEnvelopeSchema(content?.schema);
        if (contentSchema) content.schema = contentSchema;
      }
    }
  }

  return spec;
}

function unwrapEnvelopeSchema(schema: any): any {
  const override = schema?.allOf?.find((entry: any) => entry?.properties?.data);
  return override?.properties?.data;
}

export default defineConfig({
  orion: {
    input: {
      target: "../core/openapi.yaml",
      override: {
        transformer: unwrapSuccessEnvelope,
      },
    },
    output: {
      target: "./src/orion-sdk/index.ts",
      client: "react-query",
      httpClient: "fetch",
      baseUrl: { getBaseUrlFromSpecification: false, baseUrl: "" },
      override: {
        fetch: {
          includeHttpResponseReturnType: false,
        },
        mutator: {
          path: "./src/lib/custom-instance.ts",
          name: "orvalFetchClient",
        },
        query: {
          useQuery: true,
        },
      },
    },
  },
});
