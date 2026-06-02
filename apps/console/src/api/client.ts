const AUTH_TOKEN_KEY = "orion_token";

export function getApiBase(): string {
  const baseUrl =
    (import.meta as unknown as { env: { VITE_API_BASE_URL?: string } }).env.VITE_API_BASE_URL ?? "";
  const normalized = baseUrl.replace(/\/$/, "");
  return normalized.endsWith("/v1") ? normalized.slice(0, -3) : normalized;
}

export function getToken(): string | null {
  return localStorage.getItem(AUTH_TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(AUTH_TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(AUTH_TOKEN_KEY);
}

export type ApiError = Error & {
  status?: number;
  payload?: unknown;
};

type Envelope<T = unknown> = { success?: boolean; message?: string; data?: T; error?: string };

function toHeaders(headers?: HeadersInit): Record<string, string> {
  const result: Record<string, string> = {};
  if (!headers) return result;

  if (headers instanceof Headers) {
    headers.forEach((value, key) => {
      result[key] = value;
    });
    return result;
  }

  if (Array.isArray(headers)) {
    for (const [key, value] of headers) {
      result[key] = value;
    }
    return result;
  }

  return headers;
}

async function parseBody(res: Response): Promise<unknown> {
  const contentType = res.headers.get("content-type") ?? "";
  if (
    contentType.includes("text/csv") ||
    contentType.includes("application/vnd.openxmlformats") ||
    contentType.includes("application/octet-stream")
  ) {
    return res.blob();
  }
  if (contentType.includes("application/json")) {
    return res.json();
  }
  return res.text();
}

async function toApiError(res: Response): Promise<ApiError> {
  const body = (await parseBody(res).catch(() => undefined)) as Envelope | undefined;
  const message = body?.error ?? body?.message ?? `HTTP ${res.status}`;
  const error = new Error(message) as ApiError;
  error.status = res.status;
  error.payload = body;
  return error;
}

function toNetworkApiError(err: unknown): ApiError {
  if (err instanceof Error) return err as ApiError;
  return new Error("Network request failed") as ApiError;
}

export const orvalFetchClient = async <T>(endpoint: string, options?: RequestInit): Promise<T> => {
  const baseUrl = getApiBase();
  const url = endpoint.startsWith("http")
    ? endpoint
    : `${baseUrl}${endpoint.startsWith("/") ? endpoint : `/${endpoint}`}`;
  const token = getToken();
  const defaultHeaders: Record<string, string> = {
    "Content-Type": "application/json",
    "cache-control": "no-cache",
    pragma: "no-cache",
  };
  if (token) {
    defaultHeaders.Authorization = `Bearer ${token}`;
  }

  const headers = {
    ...defaultHeaders,
    ...toHeaders(options?.headers),
  };

  try {
    const res = await fetch(url, {
      ...options,
      method: options?.method ?? "GET",
      headers,
      body: options?.body !== undefined && options?.method !== "GET" ? options.body : undefined,
      signal: options?.signal,
    });

    if (res.status === 401 && !endpoint.includes("/v1/auth/login")) {
      clearToken();
      window.location.href = "/login?session=expired";
    }

    if (!res.ok) {
      throw await toApiError(res);
    }

    const body = await parseBody(res);
    if (!isEnvelope(body)) {
      return body as T;
    }
    if (body.success === false) {
      throw new Error(body.error ?? body.message ?? "Request failed");
    }
    return body.data as T;
  } catch (err) {
    throw isApiError(err) ? err : toNetworkApiError(err);
  }
};

function isEnvelope(value: unknown): value is Envelope {
  return typeof value === "object" && value !== null && "success" in value && "data" in value;
}

function isApiError(err: unknown): err is ApiError {
  return typeof (err as ApiError)?.status === "number";
}

export type ErrorType<Err> = ApiError & { info?: Err };

export async function authLogin(username: string, password: string): Promise<{ token: string }> {
  const res = await orvalFetchClient<{ token: string }>("/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.token) throw new Error("Login failed");
  setToken(res.token);
  return res;
}
