"use client";

import { useState } from "react";
import { Field, FormError, SubmitButton } from "@/components/Field";
import type { SubmitError } from "@/components/ProductForm";
import {
  MAX_MODIFIERS,
  type ModifierForm,
  type ModifierGroupForm,
  type ModifierGroupRequest,
  emptyModifier,
  moveItem,
  parseCount,
  selectionLabel,
  toGroupRequest,
  validateGroupForm,
} from "@/lib/modifier";
import type { FieldErrors } from "@/lib/validate";

interface Props {
  initial: ModifierGroupForm;
  submitLabel: string;
  onSubmit: (req: ModifierGroupRequest) => Promise<SubmitError | null>;
  onDelete?: () => Promise<string | null>;
}

const smallButton =
  "min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-sm hover:bg-stone-100 disabled:opacity-40";

export function ModifierGroupFormView({
  initial,
  submitLabel,
  onSubmit,
  onDelete,
}: Props) {
  const [form, setForm] = useState<ModifierGroupForm>(initial);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);

  const set = <K extends keyof ModifierGroupForm>(
    key: K,
    value: ModifierGroupForm[K],
  ) => setForm((f) => ({ ...f, [key]: value }));
  const setModifier = (index: number, patch: Partial<ModifierForm>) =>
    setForm((f) => ({
      ...f,
      modifiers: f.modifiers.map((m, i) =>
        i === index ? { ...m, ...patch } : m,
      ),
    }));

  const min = parseCount(form.min_select);
  const max = parseCount(form.max_select);
  const rulePreview =
    min !== null && max !== null && max >= 1 && max >= min
      ? selectionLabel(min, max)
      : null;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const local = validateGroupForm(form);
    setErrors(local);
    setFormError(undefined);
    if (Object.keys(local).length > 0) return;

    setPending(true);
    const failure = await onSubmit(toGroupRequest(form));
    setPending(false);
    if (failure) {
      setErrors(failure.fields);
      setFormError(failure.message);
    }
  }

  async function remove() {
    if (!onDelete || !window.confirm(`Hapus grup "${initial.name}"?`)) return;
    setFormError(undefined);
    const message = await onDelete();
    if (message) setFormError(message);
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <FormError message={formError} />
      <Field
        id="name"
        label="Nama grup"
        placeholder="mis. Level Gula, Topping"
        value={form.name}
        onChange={(e) => set("name", e.target.value)}
        error={errors.name}
        required
      />

      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          id="min_select"
          label="Minimal pilihan (0 = opsional)"
          inputMode="numeric"
          value={form.min_select}
          onChange={(e) => set("min_select", e.target.value)}
          error={errors.min_select}
        />
        <Field
          id="max_select"
          label="Maksimal pilihan"
          inputMode="numeric"
          value={form.max_select}
          onChange={(e) => set("max_select", e.target.value)}
          error={errors.max_select}
        />
      </div>
      {rulePreview && (
        <p className="text-sm text-stone-600">Aturan: {rulePreview}</p>
      )}

      <div className="space-y-3">
        <h2 className="text-sm font-medium text-stone-700">Opsi</h2>
        {errors.modifiers && (
          <p role="alert" className="text-sm text-red-700">
            {errors.modifiers}
          </p>
        )}
        {form.modifiers.map((m, i) => (
          <div
            key={m.key}
            className="space-y-3 rounded-xl border border-stone-200 bg-white p-3"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                id={`${m.key}-name`}
                label="Nama opsi"
                value={m.name}
                onChange={(e) => setModifier(i, { name: e.target.value })}
                error={errors[`modifiers[${i}].name`]}
                required
              />
              <Field
                id={`${m.key}-price`}
                label="Tambahan harga (Rp)"
                inputMode="numeric"
                placeholder="0 = gratis"
                value={m.price_delta}
                onChange={(e) =>
                  setModifier(i, { price_delta: e.target.value })
                }
                error={errors[`modifiers[${i}].price_delta`]}
              />
            </div>
            <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
              <label className="flex min-h-11 items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={m.is_default}
                  onChange={(e) =>
                    setModifier(i, { is_default: e.target.checked })
                  }
                  className="size-5"
                />
                Default
              </label>
              <label className="flex min-h-11 items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={m.is_active}
                  onChange={(e) =>
                    setModifier(i, { is_active: e.target.checked })
                  }
                  className="size-5"
                />
                Aktif
              </label>
              <div className="ml-auto flex gap-2">
                <button
                  type="button"
                  className={smallButton}
                  disabled={i === 0}
                  onClick={() =>
                    set("modifiers", moveItem(form.modifiers, i, i - 1))
                  }
                  aria-label={`Naikkan opsi ${i + 1}`}
                >
                  Naik
                </button>
                <button
                  type="button"
                  className={smallButton}
                  disabled={i === form.modifiers.length - 1}
                  onClick={() =>
                    set("modifiers", moveItem(form.modifiers, i, i + 1))
                  }
                  aria-label={`Turunkan opsi ${i + 1}`}
                >
                  Turun
                </button>
                {form.modifiers.length > 1 && (
                  <button
                    type="button"
                    className="min-h-11 rounded-lg border border-red-300 px-3 text-sm text-red-800 hover:bg-red-50"
                    onClick={() =>
                      set(
                        "modifiers",
                        form.modifiers.filter((_, j) => j !== i),
                      )
                    }
                  >
                    Hapus
                  </button>
                )}
              </div>
            </div>
            {errors[`modifiers[${i}].is_default`] && (
              <p className="text-sm text-red-700">
                {errors[`modifiers[${i}].is_default`]}
              </p>
            )}
          </div>
        ))}
        {form.modifiers.length < MAX_MODIFIERS && (
          <button
            type="button"
            className={`${smallButton} w-full`}
            onClick={() =>
              set("modifiers", [...form.modifiers, emptyModifier()])
            }
          >
            Tambah opsi
          </button>
        )}
      </div>

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
