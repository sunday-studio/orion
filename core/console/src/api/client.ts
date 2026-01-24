import ky from 'ky';

interface WindowWithEnv extends Window {
  __ENV__?: {
    VITE_API_BASE_URL?: string;
  };
}

const getBaseUrl = (): string => {
  // Check for environment variable first
  // Vite exposes env variables via import.meta.env at build time
  // For runtime, we'll check window.__ENV__ or use default
  if (typeof window !== 'undefined') {
    const windowWithEnv = window as WindowWithEnv;
    if (windowWithEnv.__ENV__?.VITE_API_BASE_URL) {
      return windowWithEnv.__ENV__.VITE_API_BASE_URL;
    }
  }
  // Try import.meta.env (will be replaced by Vite at build time)
  // Vite replaces import.meta.env at build time, so we need to access it dynamically
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access
  const metaEnv = (import.meta as { env?: { VITE_API_BASE_URL?: string } }).env;
  if (metaEnv?.VITE_API_BASE_URL) {
    return metaEnv.VITE_API_BASE_URL;
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
  params?: Record<string, string | number | boolean>;
  data?: unknown;
  signal?: AbortSignal;
}): Promise<T> => {
  const { method, url, params, data, signal } = config;

  return instance(url, {
    method: method as 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH',
    searchParams: params as Record<string, string | number | boolean>,
    json: data,
    signal,
  }).json<T>();
};
