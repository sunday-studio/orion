import { defineConfig } from "orval";

const HTTP_METHODS = ["get", "post", "put", "patch", "delete"] as const;
const DEFINITION_PREFIXES = [
  ["internal_api.", "api."],
  ["orion_core_internal_service.", "service."],
  ["orion_core_internal_db.", "db."],
  ["orion_core_internal_utils.", "utils."],
] as const;

function unwrapSuccessEnvelope(spec: any): any {
  normalizeDefinitionNames(spec);

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

function normalizeDefinitionNames(spec: any): void {
  renameSchemaMap(spec.definitions);
  renameSchemaMap(spec.components?.schemas);

  rewriteDefinitionRefs(spec);
}

function renameSchemaMap(schemas: Record<string, unknown> | undefined): void {
  if (!schemas) return;

  for (const name of Object.keys(schemas)) {
    const normalized = normalizeDefinitionName(name);
    if (normalized === name) continue;

    schemas[normalized] = schemas[name];
    delete schemas[name];
  }
}

function normalizeDefinitionName(name: string): string {
  return DEFINITION_PREFIXES.reduce(
    (normalized, [from, to]) => normalized.replaceAll(from, to),
    name,
  );
}

function rewriteDefinitionRefs(value: any): void {
  if (!value || typeof value !== "object") return;

  if (typeof value.$ref === "string") {
    value.$ref = normalizeDefinitionName(value.$ref);
  }

  for (const child of Object.values(value)) {
    rewriteDefinitionRefs(child);
  }
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
          path: "./src/api/client.ts",
          name: "orvalFetchClient",
        },
        query: {
          useQuery: true,
        },
      },
    },
  },
});
