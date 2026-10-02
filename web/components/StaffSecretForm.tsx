"use client";

import { useState } from "react";
import { Field, FormError } from "@/components/Field";

interface Props {
  id: string;
  label: string;
  type?: "password" | "text";
  inputMode?: "numeric";
  maxLength?: number;
  validate: (value: string) => string | undefined;
  onSubmit: (value: string) => Promise<string | null>;
  onCancel: () => void;
  extra?: React.ReactNode;
}

export function StaffSecretForm({
  id,
  label,
  type = "password",
  inputMode,
  maxLength,
  validate,
  onSubmit,
  onCancel,
  extra,
}: Props) {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string>();
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const local = validate(value);
    setError(local);
    setFormError(undefined);
    if (local) return;
    setPending(true);
    const failure = await onSubmit(value);
    setPending(false);
    if (failure) setFormError(failure);
  }

  return (
    <form
      onSubmit={submit}
      noValidate
      className="space-y-3 rounded-lg border border-amber-200 bg-amber-50 p-3"
    >
      <FormError message={formError} />
      <Field
        id={id}
        label={label}
        type={type}
        inputMode={inputMode}
        maxLength={maxLength}
        autoComplete="new-password"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        error={error}
        required
      />
      <div className="flex flex-wrap gap-2">
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
        {extra}
      </div>
    </form>
  );
}
