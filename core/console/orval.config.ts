import { defineConfig } from 'orval';

export default defineConfig({
  api: {
    input: {
      target: '../docs/swagger.yaml',
    },
    output: {
      target: './src/api/generated',
      schemas: './src/api/generated/models',
      client: 'react-query',
      mode: 'tags-split',
      override: {
        mutator: {
          path: './src/api/client.ts',
          name: 'customInstance',
        },
        query: {
          useQuery: true,
          useInfinite: false,
          useInfiniteQueryParam: 'page',
        },
      },
    },
    hooks: {
      afterAllFilesWrite: 'prettier --write',
    },
  },
});
