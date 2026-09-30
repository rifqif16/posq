"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Field, FormError, SubmitButton } from "@/components/Field";
import { ApiError, register } from "@/lib/api";
import { type FieldErrors, validateRegister } from "@/lib/validate";

export default function RegisterPage() {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [formError, setFormError] = useState<string>();
  const [errors, setErrors] = useState<FieldErrors>({});

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const input = {
      business_name: String(form.get("business_name") ?? ""),
      owner_name: String(form.get("owner_name") ?? ""),
      email: String(form.get("email") ?? ""),
      password: String(form.get("password") ?? ""),
    };

    const local = validateRegister(input);
    setErrors(local);
    setFormError(undefined);
    if (Object.keys(local).length > 0) return;

    setPending(true);
    try {
      await register(input);
      router.replace("/dashboard");
    } catch (err) {
      if (err instanceof ApiError) {
        setErrors(err.fields);
        if (Object.keys(err.fields).length === 0) setFormError(err.message);
      } else {
        setFormError("Tidak dapat terhubung ke server");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 px-4 py-8">
      <header>
        <h1 className="text-2xl font-semibold">Daftar POSQ</h1>
        <p className="text-sm text-stone-600">
          Mulai masa trial, tanpa kartu kredit.
        </p>
      </header>
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <FormError message={formError} />
        <Field
          id="business_name"
          name="business_name"
          label="Nama usaha"
          autoComplete="organization"
          error={errors.business_name}
          required
        />
        <Field
          id="owner_name"
          name="owner_name"
          label="Nama pemilik"
          autoComplete="name"
          error={errors.owner_name}
          required
        />
        <Field
          id="email"
          name="email"
          type="email"
          label="Email"
          autoComplete="email"
          error={errors.email}
          required
        />
        <Field
          id="password"
          name="password"
          type="password"
          label="Password (min. 10 karakter)"
          autoComplete="new-password"
          error={errors.password}
          required
        />
        <SubmitButton pending={pending}>Buat akun</SubmitButton>
      </form>
      <p className="text-center text-sm text-stone-600">
        Sudah punya akun?{" "}
        <Link href="/login" className="font-medium text-amber-800 underline">
          Masuk
        </Link>
      </p>
    </main>
  );
}
