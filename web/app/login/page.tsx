"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Field, FormError, SubmitButton } from "@/components/Field";
import { ApiError, login } from "@/lib/api";
import { validateEmail } from "@/lib/validate";

export default function LoginPage() {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [formError, setFormError] = useState<string>();
  const [emailError, setEmailError] = useState<string>();

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const email = String(form.get("email") ?? "");
    const password = String(form.get("password") ?? "");

    const invalid = validateEmail(email);
    setEmailError(invalid);
    setFormError(undefined);
    if (invalid) return;

    setPending(true);
    try {
      await login(email, password);
      router.replace("/dashboard");
    } catch (err) {
      setFormError(
        err instanceof ApiError
          ? err.message
          : "Tidak dapat terhubung ke server",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 px-4">
      <header>
        <h1 className="text-2xl font-semibold">Masuk ke POSQ</h1>
        <p className="text-sm text-stone-600">
          Kelola kedai Anda dari satu tempat.
        </p>
      </header>
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <FormError message={formError} />
        <Field
          id="email"
          name="email"
          type="email"
          label="Email"
          autoComplete="email"
          error={emailError}
          required
        />
        <Field
          id="password"
          name="password"
          type="password"
          label="Password"
          autoComplete="current-password"
          required
        />
        <SubmitButton pending={pending}>Masuk</SubmitButton>
      </form>
      <p className="text-center text-sm text-stone-600">
        Belum punya akun?{" "}
        <Link href="/register" className="font-medium text-amber-800 underline">
          Daftar
        </Link>
      </p>
    </main>
  );
}
