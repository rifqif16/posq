"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ProductFormView, type SubmitError } from "@/components/ProductForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { createProduct, listCategories } from "@/lib/catalog-api";
import type { Category } from "@/lib/category";
import { type ProductRequest, emptyForm } from "@/lib/product";

export default function NewProductPage() {
  const router = useRouter();
  const { profile } = useSession();
  const canWrite =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [categories, setCategories] = useState<Category[] | null>(null);
  const [loadError, setLoadError] = useState<string>();

  useEffect(() => {
    listCategories()
      .then(setCategories)
      .catch(() => setLoadError("Tidak dapat memuat kategori"));
  }, []);

  async function onSubmit(req: ProductRequest): Promise<SubmitError | null> {
    try {
      await createProduct(req);
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

  if (!canWrite)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk menambah produk.
      </p>
    );
  if (loadError)
    return (
      <p role="alert" className="text-red-800">
        {loadError}
      </p>
    );
  if (!categories) return <p className="text-stone-600">Memuat…</p>;

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Produk baru</h1>
        <Link
          href="/admin/products"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>
      <ProductFormView
        initial={emptyForm()}
        categories={categories}
        submitLabel="Simpan produk"
        onSubmit={onSubmit}
      />
    </section>
  );
}
