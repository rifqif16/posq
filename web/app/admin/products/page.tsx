"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { listCategories, listProducts } from "@/lib/catalog-api";
import type { Category } from "@/lib/category";
import { type Product, sellPriceLabel } from "@/lib/product";

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";
const DEBOUNCE_MS = 300;

export default function ProductsPage() {
  const { profile } = useSession();
  const canWrite =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [categories, setCategories] = useState<Category[]>([]);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [items, setItems] = useState<Product[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string>();
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    listCategories()
      .then(setCategories)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    let cancelled = false;
    setState("loading");
    listProducts({ q: query, category_id: categoryId })
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
  }, [query, categoryId]);

  const loadMore = useCallback(async () => {
    if (!cursor) return;
    setLoadingMore(true);
    try {
      const res = await listProducts({
        q: query,
        category_id: categoryId,
        cursor,
      });
      setItems((prev) => [...prev, ...res.items]);
      setCursor(res.next_cursor);
    } catch (e) {
      setError(message(e));
    } finally {
      setLoadingMore(false);
    }
  }, [cursor, query, categoryId]);

  const categoryName = (id: string | null) =>
    categories.find((c) => c.id === id)?.name;

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Produk</h1>
        {canWrite && (
          <div className="flex gap-2">
            <Link
              href="/admin/products/import"
              className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 py-2.5 text-sm hover:bg-stone-100"
            >
              Impor / ekspor
            </Link>
            <Link
              href="/admin/products/new"
              className="min-h-11 rounded-lg bg-amber-700 px-4 py-2.5 text-sm font-medium text-white hover:bg-amber-800"
            >
              Produk baru
            </Link>
          </div>
        )}
      </div>

      <div className="flex flex-wrap gap-2">
        <input
          type="search"
          aria-label="Cari produk"
          placeholder="Cari nama, SKU, atau barcode"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="min-h-11 min-w-56 flex-1 rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30"
        />
        <select
          aria-label="Filter kategori"
          value={categoryId}
          onChange={(e) => setCategoryId(e.target.value)}
          className="min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-base"
        >
          <option value="">Semua kategori</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.parent_id ? `— ${c.name}` : c.name}
            </option>
          ))}
        </select>
      </div>

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
          {query || categoryId
            ? "Tidak ada produk yang cocok."
            : `Belum ada produk.${canWrite ? " Tambahkan yang pertama." : ""}`}
        </p>
      )}
      {state === "ready" && items.length > 0 && (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {items.map((p) => {
            const v = p.variants[0];
            const detail =
              p.variants.length > 1 ? `${p.variants.length} varian` : v?.sku;
            const body = (
              <div className="flex items-center justify-between gap-3 p-3">
                <div className="min-w-0">
                  <p className="truncate font-medium">
                    {p.name}
                    {!p.is_active && (
                      <span className="ml-2 rounded bg-stone-200 px-1.5 py-0.5 text-xs font-normal">
                        Nonaktif
                      </span>
                    )}
                  </p>
                  <p className="truncate text-sm text-stone-600">
                    {detail}
                    {categoryName(p.category_id)
                      ? ` · ${categoryName(p.category_id)}`
                      : ""}
                  </p>
                </div>
                <p className="shrink-0 text-right font-medium">
                  {sellPriceLabel(p)}
                </p>
              </div>
            );
            return (
              <li key={p.id}>
                {canWrite ? (
                  <Link
                    href={`/admin/products/${p.id}`}
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
