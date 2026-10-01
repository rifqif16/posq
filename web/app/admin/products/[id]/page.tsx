"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ProductFormView, type SubmitError } from "@/components/ProductForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  deleteProduct,
  getProduct,
  listCategories,
  listModifierGroups,
  updateProduct,
} from "@/lib/catalog-api";
import type { Category } from "@/lib/category";
import type { ModifierGroup } from "@/lib/modifier";
import {
  type Product,
  type ProductRequest,
  formFromProduct,
} from "@/lib/product";

type Load =
  | { kind: "loading" }
  | {
      kind: "ready";
      product: Product;
      categories: Category[];
      groups: ModifierGroup[];
    }
  | { kind: "error"; message: string };

export default function EditProductPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { profile } = useSession();
  const canWrite =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [load, setLoad] = useState<Load>({ kind: "loading" });

  useEffect(() => {
    if (!canWrite) return;
    let cancelled = false;
    Promise.all([getProduct(id), listCategories(), listModifierGroups()])
      .then(
        ([product, categories, groups]) =>
          !cancelled &&
          setLoad({ kind: "ready", product, categories, groups: groups.items }),
      )
      .catch((e) => {
        if (cancelled) return;
        const notFound = e instanceof ApiError && e.status === 404;
        setLoad({
          kind: "error",
          message: notFound
            ? "Produk tidak ditemukan"
            : "Tidak dapat memuat produk",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [id, canWrite]);

  if (!canWrite)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengubah produk.
      </p>
    );
  if (load.kind === "loading") return <p className="text-stone-600">Memuat…</p>;
  if (load.kind === "error") {
    return (
      <div className="space-y-2">
        <p role="alert" className="text-red-800">
          {load.message}
        </p>
        <Link
          href="/admin/products"
          className="text-sm text-amber-800 underline"
        >
          Kembali ke daftar
        </Link>
      </div>
    );
  }

  const { product, categories, groups } = load;

  async function onSubmit(req: ProductRequest): Promise<SubmitError | null> {
    try {
      await updateProduct(product.id, product.version, req);
      router.push("/admin/products");
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

  async function onDelete(): Promise<string | null> {
    try {
      await deleteProduct(product.id);
      router.push("/admin/products");
      return null;
    } catch (e) {
      return e instanceof ApiError
        ? e.message
        : "Tidak dapat terhubung ke server";
    }
  }

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Ubah produk</h1>
        <Link
          href="/admin/products"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>
      <ProductFormView
        initial={formFromProduct(product)}
        categories={categories}
        modifierGroups={groups}
        submitLabel="Simpan perubahan"
        onSubmit={onSubmit}
        onDelete={onDelete}
      />
    </section>
  );
}
