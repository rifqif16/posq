import type { FieldErrors, RegisterInput } from "./validate";

export interface Profile {
  user: { id: string; name: string; email: string; role: string };
  tenant: {
    id: string;
    name: string;
    status: string;
    trial_ends_at: string | null;
  };
  store_ids: string[];
}

interface SessionResponse extends Profile {
  access_token: string;
  expires_in: number;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields: FieldErrors = {},
  ) {
    super(message);
  }
}

let accessToken: string | null = null;

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type"))
    headers.set("Content-Type", "application/json");
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);

  const res = await fetch(`/api${path}`, {
    ...init,
    headers,
    credentials: "same-origin",
  });
  if (res.status === 204) return undefined as T;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const fields: FieldErrors = {};
    for (const e of data?.errors ?? [])
      if (typeof e?.field === "string") fields[e.field] = e.message;
    throw new ApiError(
      res.status,
      data?.code ?? "UNKNOWN",
      data?.detail ?? "Terjadi kesalahan",
      fields,
    );
  }
  return data as T;
}

function adopt(s: SessionResponse): Profile {
  accessToken = s.access_token;
  const { access_token: _a, expires_in: _e, ...profile } = s;
  return profile;
}

export async function register(input: RegisterInput): Promise<Profile> {
  return adopt(
    await request<SessionResponse>("/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  );
}

export async function login(email: string, password: string): Promise<Profile> {
  return adopt(
    await request<SessionResponse>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  );
}

let inflightRefresh: Promise<Profile | null> | null = null;

export function restoreSession(): Promise<Profile | null> {
  inflightRefresh ??= refreshOnce().finally(() => {
    inflightRefresh = null;
  });
  return inflightRefresh;
}

async function refreshOnce(): Promise<Profile | null> {
  try {
    return adopt(
      await request<SessionResponse>("/v1/auth/refresh", { method: "POST" }),
    );
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

export async function authedRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  try {
    return await request<T>(path, init);
  } catch (e) {
    if (
      !(e instanceof ApiError) ||
      e.status !== 401 ||
      e.code !== "UNAUTHORIZED"
    )
      throw e;
    if (!(await restoreSession())) throw e;
    return request<T>(path, init);
  }
}

export async function authedDownload(
  path: string,
): Promise<{ blob: Blob; filename: string }> {
  const attempt = () => {
    const headers = new Headers();
    if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
    return fetch(`/api${path}`, { headers, credentials: "same-origin" });
  };
  let res = await attempt();
  if (res.status === 401 && (await restoreSession())) res = await attempt();
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new ApiError(
      res.status,
      data?.code ?? "UNKNOWN",
      data?.detail ?? "Terjadi kesalahan",
    );
  }
  const match = /filename="([^"]+)"/.exec(
    res.headers.get("Content-Disposition") ?? "",
  );
  return { blob: await res.blob(), filename: match?.[1] ?? "produk.csv" };
}

export async function logout(): Promise<void> {
  try {
    await request<void>("/v1/auth/logout", { method: "POST" });
  } finally {
    accessToken = null;
  }
}
