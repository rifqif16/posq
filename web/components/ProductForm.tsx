"use client";

import { useState } from "react";
import { Field, FormError, SubmitButton } from "@/components/Field";
import { type Category, buildTree } from "@/lib/category";
import {
  type ProductForm,
  type ProductRequest,
  mapServerFields,
  toRequest,
  validateForm,
} from "@/lib/product";
import type { FieldErrors } from "@/lib/validate";

export interface SubmitError {
  fields: FieldErrors;
  message?: string;
}

interface Props {
  initial: ProductForm;
  categories: Category[];
  submitLabel: string;
  onSubmit: (req: ProductRequest) => Promise<SubmitError | null>;
  onDelete?: () => Promise<string | null>;
}

const inputClass =
  "block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30";

export function ProductFormView({
  initial,
  categories,
  submitLabel,
  onSubmit,
  onDelete,
}: Props) {
  const [form, setForm] = useState<ProductForm>(initial);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);
  const tree = buildTree(categories);

  const set = <K extends keyof ProductForm>(key: K, value: ProductForm[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const local = validateForm(form);
    setErrors(local);
    setFormError(undefined);
    if (Object.keys(local).length > 0) return;

    setPending(true);
    const failure = await onSubmit(toRequest(form));
    setPending(false);
    if (failure) {
      setErrors(mapServerFields(failure.fields));
      setFormError(failure.message);
    }
  }

  async function remove() {
    if (!onDelete || !window.confirm(`Hapus produk "${initial.name}"?`)) return;
    setFormError(undefined);
    const message = await onDelete();
    if (message) setFormError(message);
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <FormError message={formError} />
      <Field
        id="name"
        label="Nama produk"
        value={form.name}
        onChange={(e) => set("name", e.target.value)}
        error={errors.name}
        required
      />

      <div className="space-y-1">
        <label
          htmlFor="category_id"
          className="block text-sm font-medium text-stone-700"
        >
          Kategori
        </label>
        <select
          id="category_id"
          value={form.category_id}
          onChange={(e) => set("category_id", e.target.value)}
          className={inputClass}
        >
          <option value="">Tanpa kategori</option>
          {tree.map((node) => (
            <optgroup key={node.id} label={node.name}>
              <option value={node.id}>{node.name}</option>
              {node.children.map((c) => (
                <option
                  key={c.id}
                  value={c.id}
                >{`${node.name} › ${c.name}`}</option>
              ))}
            </optgroup>
          ))}
        </select>
        {errors.category_id && (
          <p className="text-sm text-red-700">{errors.category_id}</p>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          id="sell_price"
          label="Harga jual (Rp)"
          inputMode="numeric"
          value={form.sell_price}
          onChange={(e) => set("sell_price", e.target.value)}
          error={errors.sell_price}
          required
        />
        <Field
          id="cost_price"
          label="Harga beli (Rp)"
          inputMode="numeric"
          value={form.cost_price}
          onChange={(e) => set("cost_price", e.target.value)}
          error={errors.cost_price}
        />
        <Field
          id="sku"
          label="SKU"
          placeholder="Kosongkan untuk otomatis"
          value={form.sku}
          onChange={(e) => set("sku", e.target.value)}
          error={errors.sku}
        />
        <Field
          id="kitchen_station"
          label="Station dapur"
          placeholder="mis. kitchen, bar"
          value={form.kitchen_station}
          onChange={(e) => set("kitchen_station", e.target.value)}
          error={errors.kitchen_station}
        />
      </div>

      <div className="space-y-1">
        <label
          htmlFor="barcodes"
          className="block text-sm font-medium text-stone-700"
        >
          Barcode (satu per baris atau pisahkan koma)
        </label>
        <textarea
          id="barcodes"
          rows={3}
          value={form.barcodes}
          onChange={(e) => set("barcodes", e.target.value)}
          aria-invalid={errors.barcodes ? true : undefined}
          className={`${inputClass} py-2`}
        />
        {errors.barcodes && (
          <p className="text-sm text-red-700">{errors.barcodes}</p>
        )}
      </div>

      <fieldset className="flex flex-wrap gap-x-6 gap-y-2">
        <legend className="sr-only">Pengaturan</legend>
        {(
          [
            ["taxable", "Kena pajak"],
            ["track_stock", "Lacak stok"],
            ["is_active", "Aktif"],
          ] as const
        ).map(([key, label]) => (
          <label key={key} className="flex min-h-11 items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form[key]}
              onChange={(e) => set(key, e.target.checked)}
              className="size-5"
            />
            {label}
          </label>
        ))}
      </fieldset>

      <div className="flex flex-wrap gap-3">
        <div className="min-w-40 flex-1">
          <SubmitButton pending={pending}>{submitLabel}</SubmitButton>
        </div>
        {onDelete && (
          <button
            type="button"
            onClick={remove}
            className="min-h-11 rounded-lg border border-red-300 px-4 text-sm text-red-800 hover:bg-red-50"
          >
            Hapus
          </button>
        )}
      </div>
    </form>
  );
}
