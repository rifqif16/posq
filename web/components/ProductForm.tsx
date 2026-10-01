"use client";

import Link from "next/link";
import { useState } from "react";
import { Field, FormError, SubmitButton } from "@/components/Field";
import { type Category, buildTree } from "@/lib/category";
import { type ModifierGroup, selectionLabel } from "@/lib/modifier";
import {
  MAX_MODIFIER_GROUPS,
  MAX_VARIANTS,
  type ProductForm,
  type ProductRequest,
  type VariantForm,
  emptyVariant,
  makeDefault,
  toRequest,
  toggleId,
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
  modifierGroups: ModifierGroup[];
  submitLabel: string;
  onSubmit: (req: ProductRequest) => Promise<SubmitError | null>;
  onDelete?: () => Promise<string | null>;
}

const inputClass =
  "block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30";
const secondaryButton =
  "min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-sm hover:bg-stone-100";

export function ProductFormView({
  initial,
  categories,
  modifierGroups,
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
  const setVariant = (index: number, patch: Partial<VariantForm>) =>
    setForm((f) => ({
      ...f,
      variants: f.variants.map((v, i) =>
        i === index ? { ...v, ...patch } : v,
      ),
    }));

  function toggleVariants(on: boolean) {
    setForm((f) => ({
      ...f,
      has_variants: on,
      // Mode bervarian butuh minimal 2 varian; tambahkan satu kosong bila baru dinyalakan.
      variants:
        on && f.variants.length < 2
          ? [...f.variants, emptyVariant()]
          : f.variants,
    }));
  }

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
      setErrors(failure.fields);
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

      <div className="grid gap-4 sm:grid-cols-2">
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
        <Field
          id="kitchen_station"
          label="Station dapur"
          placeholder="mis. kitchen, bar"
          value={form.kitchen_station}
          onChange={(e) => set("kitchen_station", e.target.value)}
          error={errors.kitchen_station}
        />
      </div>

      <label className="flex min-h-11 items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          checked={form.has_variants}
          onChange={(e) => toggleVariants(e.target.checked)}
          className="size-5"
        />
        Produk punya varian (mis. ukuran S/M/L)
      </label>

      {!form.has_variants ? (
        <>
          <VariantFields
            index={0}
            v={form.variants[0]}
            errors={errors}
            showIdentity={false}
            onChange={(p) => setVariant(0, p)}
          />
          {form.variants.length > 1 && (
            <p className="text-sm text-amber-800">
              Varian lainnya akan dihapus saat disimpan. Nyalakan opsi varian
              untuk mempertahankannya.
            </p>
          )}
        </>
      ) : (
        <div className="space-y-3">
          {errors.variants && (
            <p role="alert" className="text-sm text-red-700">
              {errors.variants}
            </p>
          )}
          {form.variants.map((v, i) => (
            <div
              key={v.key}
              className="space-y-3 rounded-xl border border-stone-200 bg-white p-3"
            >
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-medium">
                  Varian {i + 1}
                  {i === 0 && (
                    <span className="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs font-normal text-amber-900">
                      Default
                    </span>
                  )}
                </p>
                <div className="flex gap-2">
                  {i > 0 && (
                    <button
                      type="button"
                      className={secondaryButton}
                      onClick={() =>
                        set("variants", makeDefault(form.variants, i))
                      }
                    >
                      Jadikan default
                    </button>
                  )}
                  {form.variants.length > 2 && (
                    <button
                      type="button"
                      className="min-h-11 rounded-lg border border-red-300 px-3 text-sm text-red-800 hover:bg-red-50"
                      onClick={() =>
                        set(
                          "variants",
                          form.variants.filter((_, j) => j !== i),
                        )
                      }
                    >
                      Hapus varian
                    </button>
                  )}
                </div>
              </div>
              <VariantFields
                index={i}
                v={v}
                errors={errors}
                showIdentity
                onChange={(p) => setVariant(i, p)}
              />
            </div>
          ))}
          {form.variants.length < MAX_VARIANTS && (
            <button
              type="button"
              className={`${secondaryButton} w-full`}
              onClick={() =>
                set("variants", [...form.variants, emptyVariant()])
              }
            >
              Tambah varian
            </button>
          )}
        </div>
      )}

      <fieldset className="space-y-2">
        <legend className="text-sm font-medium text-stone-700">
          Grup modifier
        </legend>
        {modifierGroups.length === 0 ? (
          <p className="text-sm text-stone-600">
            Belum ada grup modifier.{" "}
            <Link
              href="/admin/modifiers/new"
              className="text-amber-800 underline"
            >
              Buat grup
            </Link>
          </p>
        ) : (
          <>
            <p className="text-sm text-stone-600">
              Urutan pilih = urutan tampil di kasir (maks. {MAX_MODIFIER_GROUPS}
              ).
            </p>
            <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
              {modifierGroups.map((g) => {
                const position = form.modifier_group_ids.indexOf(g.id);
                return (
                  <li key={g.id}>
                    <label className="flex min-h-11 items-center gap-3 px-3 py-2 text-sm">
                      <input
                        type="checkbox"
                        checked={position >= 0}
                        onChange={() =>
                          set(
                            "modifier_group_ids",
                            toggleId(form.modifier_group_ids, g.id),
                          )
                        }
                        className="size-5"
                      />
                      <span className="flex-1">
                        {g.name}
                        <span className="ml-2 text-stone-500">
                          {selectionLabel(g.min_select, g.max_select)}
                        </span>
                      </span>
                      {position >= 0 && (
                        <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs text-amber-900">
                          #{position + 1}
                        </span>
                      )}
                    </label>
                  </li>
                );
              })}
            </ul>
          </>
        )}
        {errors.modifier_group_ids && (
          <p role="alert" className="text-sm text-red-700">
            {errors.modifier_group_ids}
          </p>
        )}
      </fieldset>

      <fieldset className="flex flex-wrap gap-x-6 gap-y-2">
        <legend className="sr-only">Pengaturan produk</legend>
        {(
          [
            ["taxable", "Kena pajak"],
            ["track_stock", "Lacak stok"],
            ["is_active", "Produk aktif"],
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

interface VariantFieldsProps {
  index: number;
  v: VariantForm;
  errors: FieldErrors;
  showIdentity: boolean; // nama varian dan status aktif hanya pada mode bervarian
  onChange: (patch: Partial<VariantForm>) => void;
}

function VariantFields({
  index,
  v,
  errors,
  showIdentity,
  onChange,
}: VariantFieldsProps) {
  const err = (field: string) => errors[`variants[${index}].${field}`];
  const id = (field: string) => `${v.key}-${field}`;

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        {showIdentity && (
          <Field
            id={id("name")}
            label="Nama varian"
            value={v.name}
            onChange={(e) => onChange({ name: e.target.value })}
            error={err("name")}
            required
          />
        )}
        <Field
          id={id("sell_price")}
          label="Harga jual (Rp)"
          inputMode="numeric"
          value={v.sell_price}
          onChange={(e) => onChange({ sell_price: e.target.value })}
          error={err("sell_price")}
          required
        />
        <Field
          id={id("cost_price")}
          label="Harga beli (Rp)"
          inputMode="numeric"
          value={v.cost_price}
          onChange={(e) => onChange({ cost_price: e.target.value })}
          error={err("cost_price")}
        />
        <Field
          id={id("sku")}
          label="SKU"
          placeholder="Kosongkan untuk otomatis"
          value={v.sku}
          onChange={(e) => onChange({ sku: e.target.value })}
          error={err("sku")}
        />
      </div>
      <div className="space-y-1">
        <label
          htmlFor={id("barcodes")}
          className="block text-sm font-medium text-stone-700"
        >
          Barcode (satu per baris atau pisahkan koma)
        </label>
        <textarea
          id={id("barcodes")}
          rows={2}
          value={v.barcodes}
          onChange={(e) => onChange({ barcodes: e.target.value })}
          aria-invalid={err("barcodes") ? true : undefined}
          className={`${inputClass} py-2`}
        />
        {err("barcodes") && (
          <p className="text-sm text-red-700">{err("barcodes")}</p>
        )}
      </div>
      {showIdentity && (
        <label className="flex min-h-11 items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={v.is_active}
            onChange={(e) => onChange({ is_active: e.target.checked })}
            className="size-5"
          />
          Varian aktif
        </label>
      )}
    </div>
  );
}
