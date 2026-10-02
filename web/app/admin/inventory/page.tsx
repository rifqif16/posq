"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { StockMovementForm } from "@/components/StockMovementForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  createStockMovement,
  listStockLevels,
  submitStockCounts,
} from "@/lib/inventory-api";
import {
  type MovementRequest,
  type StockLevel,
  collectCounts,
  displayQty,
  isNegative,
  itemLabel,
} from "@/lib/stock";

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";
const DEBOUNCE_MS = 300;

export default function InventoryPage() {
  const { profile } = useSession();
  const canAdjust =
    profile.user.role === "owner" || profile.user.role === "admin";
  const storeId = profile.store_ids[0];

  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<StockLevel[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [loadingMore, setLoadingMore] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [counting, setCounting] = useState(false);
  const [counts, setCounts] = useState<Record<string, string>>({});
  const [countErrors, setCountErrors] = useState<Record<string, string>>({});
  const [savingCounts, setSavingCounts] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    if (!storeId) return;
    let cancelled = false;
    setState("loading");
    listStockLevels(storeId, { q: query })
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
  }, [storeId, query]);

  const loadMore = useCallback(async () => {
    if (!storeId || !cursor) return;
    setLoadingMore(true);
    try {
      const res = await listStockLevels(storeId, { q: query, cursor });
      setItems((prev) => [...prev, ...res.items]);
      setCursor(res.next_cursor);
    } catch (e) {
      setError(message(e));
    } finally {
      setLoadingMore(false);
    }
  }, [storeId, cursor, query]);

  async function saveMovement(req: MovementRequest) {
    try {
      const res = await createStockMovement(req);
      setItems((prev) =>
        prev.map((l) =>
          l.variant_id === req.variant_id
            ? { ...l, qty_on_hand: res.qty_on_hand }
            : l,
        ),
      );
      setActiveId(null);
      setNotice("Pergerakan stok tersimpan");
      return null;
    } catch (e) {
      if (e instanceof ApiError)
        return {
          fields: e.fields,
          message: Object.keys(e.fields).length === 0 ? e.message : undefined,
        };
      return { fields: {}, message: "Tidak dapat terhubung ke server" };
    }
  }

  async function saveCounts() {
    if (!storeId) return;
    const { items: toSend, errors } = collectCounts(counts);
    setCountErrors(errors);
    setError(undefined);
    if (Object.keys(errors).length > 0) return;
    if (toSend.length === 0) {
      setError("Isi jumlah hitung fisik minimal pada satu baris");
      return;
    }
    setSavingCounts(true);
    try {
      const res = await submitStockCounts(storeId, toSend);
      const counted = new Map(
        res.results.map((r) => [r.variant_id, r.counted]),
      );
      setItems((prev) =>
        prev.map((l) =>
          counted.has(l.variant_id)
            ? { ...l, qty_on_hand: counted.get(l.variant_id) as string }
            : l,
        ),
      );
      setCounts({});
      setCounting(false);
      setNotice(
        `Hitung fisik tersimpan: ${res.adjusted} dari ${toSend.length} varian disesuaikan`,
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setSavingCounts(false);
    }
  }

  function toggleCounting() {
    setCounting((c) => !c);
    setCounts({});
    setCountErrors({});
    setActiveId(null);
    setError(undefined);
  }

  if (!storeId)
    return (
      <p role="alert" className="text-red-800">
        Akun ini belum terhubung ke outlet.
      </p>
    );

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Stok</h1>
        {canAdjust && (
          <div className="flex gap-2">
            <Link
              href="/admin/inventory/movements"
              className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 py-2.5 text-sm hover:bg-stone-100"
            >
              Riwayat
            </Link>
            <button
              onClick={toggleCounting}
              className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
            >
              {counting ? "Batal hitung fisik" : "Hitung fisik"}
            </button>
          </div>
        )}
      </div>

      <input
        type="search"
        aria-label="Cari stok"
        placeholder="Cari nama, SKU, atau barcode"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30"
      />

      {notice && (
        <p
          role="status"
          className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-900"
        >
          {notice}
        </p>
      )}
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
          {query
            ? "Tidak ada yang cocok."
            : 'Belum ada produk dengan pelacakan stok. Aktifkan "Lacak stok" pada produk.'}
        </p>
      )}
      {state === "ready" && items.length > 0 && (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {items.map((l) => (
            <li key={l.variant_id} className="space-y-2 p-3">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-medium">{itemLabel(l)}</p>
                  <p className="truncate text-sm text-stone-600">{l.sku}</p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <p
                    className={`text-right font-medium ${isNegative(l.qty_on_hand) ? "text-red-700" : ""}`}
                  >
                    {displayQty(l.qty_on_hand)}
                  </p>
                  {canAdjust && counting && (
                    <div>
                      <input
                        aria-label={`Hitung fisik ${itemLabel(l)}`}
                        inputMode="decimal"
                        placeholder="Hitung"
                        value={counts[l.variant_id] ?? ""}
                        onChange={(e) =>
                          setCounts((c) => ({
                            ...c,
                            [l.variant_id]: e.target.value,
                          }))
                        }
                        aria-invalid={
                          countErrors[l.variant_id] ? true : undefined
                        }
                        className="min-h-11 w-24 rounded-lg border border-stone-300 bg-white px-2 text-right text-base"
                      />
                    </div>
                  )}
                  {canAdjust && !counting && (
                    <button
                      onClick={() =>
                        setActiveId(
                          activeId === l.variant_id ? null : l.variant_id,
                        )
                      }
                      className="min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-sm hover:bg-stone-100"
                    >
                      Ubah stok
                    </button>
                  )}
                </div>
              </div>
              {countErrors[l.variant_id] && (
                <p className="text-right text-sm text-red-700">
                  {countErrors[l.variant_id]}
                </p>
              )}
              {canAdjust && !counting && activeId === l.variant_id && (
                <StockMovementForm
                  storeId={storeId}
                  variantId={l.variant_id}
                  onSubmit={saveMovement}
                  onCancel={() => setActiveId(null)}
                />
              )}
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
      {counting && (
        <div className="sticky bottom-3 flex items-center justify-between gap-3 rounded-xl border border-amber-300 bg-amber-50 p-3">
          <p className="text-sm text-amber-900">
            Isi hanya baris yang dihitung; baris kosong dilewati.
          </p>
          <button
            onClick={saveCounts}
            disabled={savingCounts}
            className="min-h-11 shrink-0 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800 disabled:opacity-60"
          >
            {savingCounts ? "Menyimpan…" : "Simpan hasil hitung"}
          </button>
        </div>
      )}
    </section>
  );
}
