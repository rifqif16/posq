"use client";

import { useState } from "react";
import { Field, FormError } from "@/components/Field";
import {
  KIND_LABELS,
  type MovementForm,
  type MovementKind,
  type MovementRequest,
  emptyMovementForm,
  toMovementRequest,
  validateMovementForm,
} from "@/lib/stock";
import type { FieldErrors } from "@/lib/validate";

interface Props {
  storeId: string;
  variantId: string;
  onSubmit: (
    req: MovementRequest,
  ) => Promise<{ fields: FieldErrors; message?: string } | null>;
  onCancel: () => void;
}

export function StockMovementForm({
  storeId,
  variantId,
  onSubmit,
  onCancel,
}: Props) {
  const [form, setForm] = useState<MovementForm>(emptyMovementForm());
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);

  const set = <K extends keyof MovementForm>(key: K, value: MovementForm[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const local = validateMovementForm(form);
    setErrors(local);
    setFormError(undefined);
    if (Object.keys(local).length > 0) return;
    setPending(true);
    const failure = await onSubmit(toMovementRequest(storeId, variantId, form));
    setPending(false);
    if (failure) {
      const { qty_delta, ...rest } = failure.fields;
      setErrors(qty_delta ? { ...rest, qty: qty_delta } : rest);
      setFormError(failure.message);
    }
  }

  return (
    <form
      onSubmit={submit}
      noValidate
      className="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3"
    >
      <FormError message={formError} />
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1">
          <label
            htmlFor={`${variantId}-kind`}
            className="block text-sm font-medium text-stone-700"
          >
            Jenis
          </label>
          <select
            id={`${variantId}-kind`}
            value={form.kind}
            onChange={(e) => set("kind", e.target.value as MovementKind)}
            className="block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base"
          >
            {(Object.keys(KIND_LABELS) as MovementKind[]).map((k) => (
              <option key={k} value={k}>
                {KIND_LABELS[k]}
              </option>
            ))}
          </select>
        </div>
        <Field
          id={`${variantId}-qty`}
          label="Jumlah"
          inputMode="decimal"
          value={form.qty}
          onChange={(e) => set("qty", e.target.value)}
          error={errors.qty}
          required
        />
        {form.kind === "receive" && (
          <Field
            id={`${variantId}-cost`}
            label="Harga satuan (Rp, opsional)"
            inputMode="numeric"
            value={form.unit_cost}
            onChange={(e) => set("unit_cost", e.target.value)}
            error={errors.unit_cost}
          />
        )}
        <Field
          id={`${variantId}-reason`}
          label={form.kind === "receive" ? "Catatan (opsional)" : "Alasan"}
          value={form.reason}
          onChange={(e) => set("reason", e.target.value)}
          error={errors.reason}
          required={form.kind !== "receive"}
        />
      </div>
      <div className="flex gap-2">
        <button
          disabled={pending}
          className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800 disabled:opacity-60"
        >
          {pending ? "Menyimpan…" : "Simpan"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
        >
          Batal
        </button>
      </div>
    </form>
  );
}
