"use client";

import { useState } from "react";
import { Field, FormError } from "@/components/Field";
import {
  type StaffForm,
  type StaffRole,
  type StoreInfo,
  roleLabel,
  toggleStore,
  validateStaffForm,
} from "@/lib/staff";
import type { FieldErrors } from "@/lib/validate";

interface Props {
  mode: "create" | "edit";
  initial: StaffForm;
  stores: StoreInfo[];
  roles: StaffRole[];
  submitLabel: string;
  onSubmit: (
    form: StaffForm,
  ) => Promise<{ fields: FieldErrors; message?: string } | null>;
  onCancel: () => void;
}

export function StaffFormView({
  mode,
  initial,
  stores,
  roles,
  submitLabel,
  onSubmit,
  onCancel,
}: Props) {
  const [form, setForm] = useState<StaffForm>(initial);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);
  const set = <K extends keyof StaffForm>(key: K, value: StaffForm[K]) =>
    setForm((f) => ({ ...f, [key]: value }));
  const id = (name: string) => `${mode}-${initial.email || "new"}-${name}`;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const local = validateStaffForm(form, mode);
    setErrors(local);
    setFormError(undefined);
    if (Object.keys(local).length > 0) return;
    setPending(true);
    const failure = await onSubmit(form);
    setPending(false);
    if (failure) {
      setErrors(failure.fields);
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
        <Field
          id={id("name")}
          label="Nama"
          value={form.name}
          onChange={(e) => set("name", e.target.value)}
          error={errors.name}
          required
        />
        {mode === "create" ? (
          <Field
            id={id("email")}
            label="Email"
            type="email"
            value={form.email}
            onChange={(e) => set("email", e.target.value)}
            error={errors.email}
            required
          />
        ) : (
          <Field
            id={id("email")}
            label="Email"
            value={form.email}
            disabled
            readOnly
          />
        )}
        {mode === "create" && (
          <Field
            id={id("password")}
            label="Password (min. 10 karakter)"
            type="password"
            autoComplete="new-password"
            value={form.password}
            onChange={(e) => set("password", e.target.value)}
            error={errors.password}
            required
          />
        )}
        {mode === "create" && (
          <Field
            id={id("pin")}
            label="PIN kasir 6 digit (opsional)"
            inputMode="numeric"
            maxLength={6}
            value={form.pin}
            onChange={(e) => set("pin", e.target.value)}
            error={errors.pin}
          />
        )}
        <div className="space-y-1">
          <label
            htmlFor={id("role")}
            className="block text-sm font-medium text-stone-700"
          >
            Role
          </label>
          <select
            id={id("role")}
            value={form.role}
            onChange={(e) => set("role", e.target.value as StaffRole)}
            className="block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base"
          >
            {roles.map((r) => (
              <option key={r} value={r}>
                {roleLabel(r)}
              </option>
            ))}
          </select>
        </div>
      </div>
      <fieldset className="space-y-1">
        <legend className="text-sm font-medium text-stone-700">Outlet</legend>
        <div className="flex flex-wrap gap-x-4">
          {stores.map((s) => (
            <label
              key={s.id}
              className="flex min-h-11 items-center gap-2 text-sm"
            >
              <input
                type="checkbox"
                checked={form.store_ids.includes(s.id)}
                onChange={() =>
                  set("store_ids", toggleStore(form.store_ids, s.id))
                }
                className="size-5"
              />
              {s.name}
            </label>
          ))}
        </div>
        {errors.store_ids && (
          <p role="alert" className="text-sm text-red-700">
            {errors.store_ids}
          </p>
        )}
      </fieldset>
      {mode === "edit" && (
        <label className="flex min-h-11 items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.active}
            onChange={(e) => set("active", e.target.checked)}
            className="size-5"
          />
          Akun aktif (nonaktif akan mengeluarkan dari semua perangkat)
        </label>
      )}
      <div className="flex gap-2">
        <button
          disabled={pending}
          className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800 disabled:opacity-60"
        >
          {pending ? "Menyimpan…" : submitLabel}
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
