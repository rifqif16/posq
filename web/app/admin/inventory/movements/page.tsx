"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { listStockMovements } from "@/lib/inventory-api";
import { formatDateTime } from "@/lib/price-history";
import {
  type MovementType,
  type StockMovement,
  formatSigned,
  isNegative,
  itemLabel,
  movementTypeLabel,
} from "@/lib/stock";
import { formatRupiah } from "@/lib/money";

const TYPES: MovementType[] = [
  "purchase_receive",
  "waste",
  "adjustment",
  "opname",
];
const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";

export default function MovementsPage() {
  const { profile } = useSession();
  const canView =
    profile.user.role === "owner" || profile.user.role === "admin";
  const storeId = profile.store_ids[0];
  const [type, setType] = useState<MovementType | "">("");
  const [items, setItems] = useState<StockMovement[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string>();
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    if (!canView || !storeId) return;
    let cancelled = false;
    setState("loading");
    listStockMovements(storeId, { type: type || undefined })
      .then((res) => {
        if (cancelled) return;
        setItems(res.items);
        setCursor(res.next_cursor);
        setState("ready");
      })
      .catch((e) => {
        if (cancelled) return;
        setError(message(e));
        setState("error");
      });
    return () => {
      cancelled = true;
    };
  }, [canView, storeId, type]);

  const loadMore = useCallback(async () => {
    if (!storeId || !cursor) return;
    setLoadingMore(true);
    try {
      const res = await listStockMovements(storeId, {
        type: type || undefined,
        cursor,
      });
      setItems((prev) => [...prev, ...res.items]);
      setCursor(res.next_cursor);
    } catch (e) {
      setError(message(e));
    } finally {
      setLoadingMore(false);
    }
  }, [storeId, cursor, type]);

  if (!canView)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk melihat riwayat stok.
      </p>
    );
  if (!storeId)
    return (
      <p role="alert" className="text-red-800">
        Akun ini belum terhubung ke outlet.
      </p>
    );

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Riwayat stok</h1>
        <Link
          href="/admin/inventory"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>
      <select
        aria-label="Filter jenis"
        value={type}
        onChange={(e) => setType(e.target.value as MovementType | "")}
        className="min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-base"
      >
        <option value="">Semua jenis</option>
        {TYPES.map((t) => (
          <option key={t} value={t}>
            {movementTypeLabel(t)}
          </option>
        ))}
      </select>

      {error && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
        >
          {error}
        </p>
      )}
      {state === "loading" && <p className="text-stone-600">Memuat…</p>}
      {state === "ready" && items.length === 0 && (
        <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
          Belum ada pergerakan stok.
        </p>
      )}
      {state === "ready" && items.length > 0 && (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {items.map((m) => (
            <li key={m.id} className="space-y-1 p-3">
              <div className="flex items-center justify-between gap-3">
                <p className="truncate font-medium">{itemLabel(m)}</p>
                <p
                  className={`shrink-0 font-medium ${isNegative(m.qty_delta) ? "text-red-700" : "text-green-700"}`}
                >
                  {formatSigned(m.qty_delta)}
                </p>
              </div>
              <p className="text-sm text-stone-700">
                {movementTypeLabel(m.type)}
                {m.unit_cost !== null &&
                  ` · ${formatRupiah(m.unit_cost)}/satuan`}
                {m.reason && ` · ${m.reason}`}
              </p>
              <p className="text-xs text-stone-500">
                {formatDateTime(m.created_at)} · {m.created_by.name}
              </p>
            </li>
          ))}
        </ul>
      )}
      {cursor && state === "ready" && (
        <button
          onClick={loadMore}
          disabled={loadingMore}
          className="min-h-11 w-full rounded-lg border border-stone-300 bg-white text-sm hover:bg-stone-100 disabled:opacity-60"
        >
          {loadingMore ? "Memuat…" : "Muat lebih banyak"}
        </button>
      )}
    </section>
  );
}
