"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { type Profile, logout, restoreSession } from "@/lib/api";

type State =
  | { kind: "loading" }
  | { kind: "ready"; profile: Profile }
  | { kind: "error"; message: string };

export default function DashboardPage() {
  const router = useRouter();
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    restoreSession()
      .then((profile) => {
        if (cancelled) return;
        if (!profile) router.replace("/login");
        else setState({ kind: "ready", profile });
      })
      .catch(
        () =>
          !cancelled &&
          setState({
            kind: "error",
            message: "Tidak dapat terhubung ke server",
          }),
      );
    return () => {
      cancelled = true;
    };
  }, [router]);

  async function onLogout() {
    await logout().catch(() => undefined);
    router.replace("/login");
  }

  if (state.kind === "loading") {
    return <main className="p-6 text-stone-600">Memuat…</main>;
  }
  if (state.kind === "error") {
    return (
      <main className="p-6">
        <p role="alert" className="text-red-800">
          {state.message}
        </p>
      </main>
    );
  }

  const { user, tenant, store_ids } = state.profile;
  const trialEnd = tenant.trial_ends_at
    ? new Date(tenant.trial_ends_at).toLocaleDateString("id-ID", {
        dateStyle: "long",
      })
    : null;

  return (
    <main className="mx-auto max-w-2xl space-y-6 p-6">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">{tenant.name}</h1>
          <p className="text-sm text-stone-600">
            {user.name} · {user.role}
          </p>
        </div>
        <button
          onClick={onLogout}
          className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
        >
          Keluar
        </button>
      </header>
      <dl className="grid grid-cols-2 gap-4 rounded-xl border border-stone-200 bg-white p-4 text-sm">
        <div>
          <dt className="text-stone-500">Status</dt>
          <dd className="font-medium">{tenant.status}</dd>
        </div>
        <div>
          <dt className="text-stone-500">Trial berakhir</dt>
          <dd className="font-medium">{trialEnd ?? "-"}</dd>
        </div>
        <div>
          <dt className="text-stone-500">Email</dt>
          <dd className="font-medium">{user.email}</dd>
        </div>
        <div>
          <dt className="text-stone-500">Outlet</dt>
          <dd className="font-medium">{store_ids.length}</dd>
        </div>
      </dl>
    </main>
  );
}
