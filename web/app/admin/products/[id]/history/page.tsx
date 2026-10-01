"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { getProduct } from "@/lib/catalog-api";
import { type PriceChange, describeChange, formatDateTime, showVariantName } from "@/lib/price-history";
import { listPriceHistory } from "@/lib/price-history-api";

type Load =
  | { kind: "loading" }
  | { kind: "ready"; productName: string; items: PriceChange[]; cursor: string | null }
  | { kind: "error"; message: string };

const failure = (e: unknown) => {
  if (e instanceof ApiError && e.status === 404) return "Produk tidak ditemukan";
  if (e instanceof ApiError && e.status === 403) return "Anda tidak memiliki izin untuk melihat riwayat harga";
  return "Tidak dapat memuat riwayat harga";
};

export default function PriceHistoryPage() {
  const { id } = useParams<{ id: string }>();
  const { profile } = useSession();
  const canView = profile.user.role === "owner" || profile.user.role === "admin";
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<string>();

  useEffect(() => {
    if (!canView) return;
    let cancelled = false;
    Promise.all([getProduct(id), listPriceHistory(id)])
      .then(([product, page]) => {
        if (!cancelled) setLoad({ kind: "ready", productName: product.name, items: page.items, cursor: page.next_cursor });
      })
      .catch((e) => !cancelled && setLoad({ kind: "error", message: failure(e) }));
    return () => {
      cancelled = true;
    };
  }, [id, canView]);

  const loadMore = useCallback(async () => {
    if (load.kind !== "ready" || !load.cursor) return;
    setLoadingMore(true);
    setMoreError(undefined);
    try {
      const page = await listPriceHistory(id, load.cursor);
      setLoad({ ...load, items: [...load.items, ...page.items], cursor: page.next_cursor });
    } catch (e) {
      setMoreError(failure(e));
    } finally {
      setLoadingMore(false);
    }
  }, [id, load]);

  if (!canView) return <p className="text-stone-700">Anda tidak memiliki izin untuk melihat riwayat harga.</p>;
  if (load.kind === "loading") return <p className="text-stone-600">Memuat…</p>;
  if (load.kind === "error") {
    return (
      <div className="space-y-2">
        <p role="alert" className="text-red-800">{load.message}</p>
        <Link href="/admin/products" className="text-sm text-amber-800 underline">Kembali ke daftar</Link>
      </div>
    );
  }

  const withVariant = showVariantName(load.items);

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Riwayat harga: {load.productName}</h1>
        <Link href={`/admin/products/${id}`} className="text-sm text-amber-800 underline">Kembali</Link>
      </div>
      {load.items.length === 0 ? (
        <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
          Belum ada perubahan harga.
        </p>
      ) : (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {load.items.map((c) => {
            const v = describeChange(c);
            return (
              <li key={c.id} className="space-y-1 p-3">
                <div className="flex items-center justify-between gap-3">
                  <p className="font-medium">
                    {v.fieldLabel}
                    {withVariant && <span className="ml-2 font-normal text-stone-600">· {c.variant_name}</span>}
                  </p>
                  <p className={`shrink-0 text-sm font-medium ${v.direction === "naik" ? "text-red-700" : "text-green-700"}`}>
                    {v.difference}
                  </p>
                </div>
                <p className="text-sm">{v.from} → {v.to}</p>
                <p className="text-xs text-stone-500">
                  {formatDateTime(c.at)} · {c.changed_by.name}
                </p>
              </li>
            );
          })}
        </ul>
      )}
      {moreError && <p role="alert" className="text-sm text-red-800">{moreError}</p>}
      {load.cursor && (
        <button onClick={loadMore} disabled={loadingMore} className="min-h-11 w-full rounded-lg border border-stone-300 bg-white text-sm hover:bg-stone-100 disabled:opacity-60">
          {loadingMore ? "Memuat…" : "Muat lebih banyak"}
        </button>
      )}
    </section>
  );
}
