// Klien API auth. Access token hanya disimpan di memori (bukan localStorage);
// sesi dipulihkan lewat refresh token di cookie HttpOnly.
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
  if (init.body) headers.set("Content-Type", "application/json");
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
    for (const e of data?.errors ?? []) fields[e.field] = e.message;
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

// Memulihkan sesi dari cookie refresh; mengembalikan null bila belum login.
export async function restoreSession(): Promise<Profile | null> {
  try {
    return adopt(
      await request<SessionResponse>("/v1/auth/refresh", { method: "POST" }),
    );
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

export async function logout(): Promise<void> {
  try {
    await request<void>("/v1/auth/logout", { method: "POST" });
  } finally {
    accessToken = null;
  }
}
