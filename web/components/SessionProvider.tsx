"use client";

import { useRouter } from "next/navigation";
import { createContext, useContext, useEffect, useState } from "react";
import { type Profile, logout as apiLogout, restoreSession } from "@/lib/api";

interface SessionValue {
  profile: Profile;
  logout: () => Promise<void>;
}

const SessionContext = createContext<SessionValue | null>(null);

export function useSession(): SessionValue {
  const value = useContext(SessionContext);
  if (!value)
    throw new Error("useSession harus dipakai di dalam SessionProvider");
  return value;
}

type State =
  | { kind: "loading" }
  | { kind: "ready"; profile: Profile }
  | { kind: "error" };

export function SessionProvider({
  children,
  loginPath = "/login",
}: {
  children: React.ReactNode;
  loginPath?: string;
}) {
  const router = useRouter();
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    restoreSession()
      .then((profile) => {
        if (cancelled) return;
        if (profile) setState({ kind: "ready", profile });
        else router.replace(loginPath);
      })
      .catch(() => !cancelled && setState({ kind: "error" }));
    return () => {
      cancelled = true;
    };
  }, [router, loginPath]);

  async function logout() {
    await apiLogout().catch(() => undefined);
    router.replace(loginPath);
  }

  if (state.kind === "loading")
    return <main className="p-6 text-stone-600">Memuat…</main>;
  if (state.kind === "error") {
    return (
      <main className="p-6">
        <p role="alert" className="text-red-800">
          Tidak dapat terhubung ke server
        </p>
      </main>
    );
  }
  return (
    <SessionContext.Provider value={{ profile: state.profile, logout }}>
      {children}
    </SessionContext.Provider>
  );
}
