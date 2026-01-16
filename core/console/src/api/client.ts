import ky from 'ky';

const getBaseUrl = () => {
  // Check for environment variable first
  // In Vite, we'll use a different approach for build-time
  if (typeof process !== 'undefined' && process.env?.VITE_API_BASE_URL) {
    return process.env.VITE_API_BASE_URL;
  }
  // Default to local development server
  return 'http://localhost:8999';
};

const instance = ky.create({
  prefixUrl: getBaseUrl(),
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 30000,
  retry: {
    limit: 2,
    methods: ['get'],
    statusCodes: [408, 413, 429, 500, 502, 503, 504],
  },
});

export const customInstance = async <T>(config: {
  method: string;
  url: string;
  params?: Record<string, unknown>;
  data?: unknown;
  signal?: AbortSignal;
}): Promise<T> => {
  const { method, url, params, data, signal } = config;

  return instance(url, {
    method: method as any,
    searchParams: params,
    json: data,
    signal,
  }).json<T>();
};
