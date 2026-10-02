"use client";

import { useSession } from "@/components/SessionProvider";

export default function CashierHomePage() {
  const { profile, logout } = useSession();

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 px-4">
      <header>
        <h1 className="text-2xl font-semibold">Halo, {profile.user.name}</h1>
        <p className="text-sm text-stone-600">
          {profile.tenant.name} · {profile.user.role}
        </p>
      </header>
      <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
        Layar kasir akan hadir pada fase berikutnya.
      </p>
      <button
        onClick={logout}
        className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
      >
        Keluar
      </button>
    </main>
  );
}
