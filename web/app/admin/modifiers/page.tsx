"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { listModifierGroups } from "@/lib/catalog-api";
import { type ModifierGroup, selectionLabel } from "@/lib/modifier";

type Load =
  | { kind: "loading" }
  | { kind: "ready"; groups: ModifierGroup[] }
  | { kind: "error"; message: string };

export default function ModifierGroupsPage() {
  const { profile } = useSession();
  const canManage =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [load, setLoad] = useState<Load>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    listModifierGroups()
      .then(
        (res) => !cancelled && setLoad({ kind: "ready", groups: res.items }),
      )
      .catch(
        (e) =>
          !cancelled &&
          setLoad({
            kind: "error",
            message:
              e instanceof ApiError
                ? e.message
                : "Tidak dapat terhubung ke server",
          }),
      );
    return () => {
      cancelled = true;
    };
  }, []);

  if (load.kind === "loading") return <p className="text-stone-600">Memuat…</p>;
  if (load.kind === "error")
    return (
      <p role="alert" className="text-red-800">
        {load.message}
      </p>
    );

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Grup modifier</h1>
        {canManage && (
          <Link
            href="/admin/modifiers/new"
            className="min-h-11 rounded-lg bg-amber-700 px-4 py-2.5 text-sm font-medium text-white hover:bg-amber-800"
          >
            Grup baru
          </Link>
        )}
      </div>
      {load.groups.length === 0 ? (
        <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
          Belum ada grup modifier.
          {canManage ? " Buat yang pertama, mis. Level Gula atau Topping." : ""}
        </p>
      ) : (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {load.groups.map((g) => {
            const body = (
              <div className="space-y-1 p-3">
                <div className="flex items-center justify-between gap-3">
                  <p className="truncate font-medium">{g.name}</p>
                  <p className="shrink-0 text-sm text-stone-600">
                    {selectionLabel(g.min_select, g.max_select)}
                  </p>
                </div>
                <p className="truncate text-sm text-stone-600">
                  {g.modifiers.map((m) => m.name).join(" · ")} (
                  {g.modifiers.length} opsi)
                </p>
              </div>
            );
            return (
              <li key={g.id}>
                {canManage ? (
                  <Link
                    href={`/admin/modifiers/${g.id}`}
                    className="block hover:bg-stone-50"
                  >
                    {body}
                  </Link>
                ) : (
                  body
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
