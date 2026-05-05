const AUTH_TOKEN_KEY = "orion_token";

/** API base: VITE_API_BASE_URL (trimmed) when set, else "/v1" so SPA works when served from core without .env. */
function getApiBase(): string {
  return "http://localhost:8999/v1";
  // const b = (import.meta as unknown as { env: { VITE_API_BASE_URL?: string } }).env.VITE_API_BASE_URL ?? "";
  // return b ? b.replace(/\/$/, "") : "/v1";
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

type Envelope = { success?: boolean; message?: string; data?: unknown; error?: string };

/**
 * Custom mutator for Orval's fetch client. Receives (url, options) as the fetch
 * client passes. Returns { data, status, headers } where data is the parsed
 * envelope so generated types (expecting data: XxxResponse) match.
 * Sends Bearer token when orion_token exists. On 401, clears token and redirects to /login.
 */
export const customInstance = async <T>(url: string, options?: RequestInit): Promise<T> => {
  const prefix = getApiBase();
  const full = url.startsWith("http") ? url : `${prefix}${url.startsWith("/") ? url : `/${url}`}`;
  const headers = new Headers(options?.headers as HeadersInit);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(full, {
    ...options,
    headers,
    method: (options?.method as string) || "GET",
  });

  if (res.status === 401) {
    clearToken();
    window.location.href = "/login";
    const body = (await res.json().catch(() => ({}))) as Envelope;
    throw new Error(body.error ?? body.message ?? "Unauthorized");
  }

  const body = (await res.json()) as Envelope;
  if (!res.ok) throw new Error(body.error ?? body.message ?? `HTTP ${res.status}`);
  if (body.success === false) throw new Error(body.error ?? body.message ?? "Request failed");
  return { data: body, status: res.status, headers: res.headers } as T;
};

export type ErrorType<Err> = Error & { info?: Err };

/** Unwrap envelope: r?.data?.data ?? null. Use for Orval hook responses. Pass type arg, e.g. dataOf<GetAgentDetailResponseData>(res). Param is `any` to accept Orval union (4xx/error) types. */
export function dataOf<T>(r: any): T | null {
  return r?.data?.data ?? null;
}

/** POST /v1/auth/login. On success sets orion_token and returns { token }. On failure throws. */
export async function authLogin(username: string, password: string): Promise<{ token: string }> {
  const url = `${getApiBase()}/auth/login`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const env = (await res.json()) as Envelope & { data?: { token?: string } };
  if (!res.ok) throw new Error(env.error ?? env.message ?? `HTTP ${res.status}`);
  if (env.success !== true || !env.data?.token) throw new Error(env.message ?? "Login failed");
  setToken(env.data.token);
  return { token: env.data.token };
}
