import type { InputHTMLAttributes } from "react";

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
}

export function Field({ label, error, id, ...input }: FieldProps) {
  const errId = `${id}-error`;
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="block text-sm font-medium text-stone-700">
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errId : undefined}
        className="block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30 aria-invalid:border-red-600"
        {...input}
      />
      {error && (
        <p id={errId} className="text-sm text-red-700">
          {error}
        </p>
      )}
    </div>
  );
}

export function FormError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p
      role="alert"
      className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
    >
      {message}
    </p>
  );
}

export function SubmitButton({
  pending,
  children,
}: {
  pending: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="submit"
      disabled={pending}
      className="min-h-11 w-full rounded-lg bg-amber-700 px-4 font-medium text-white hover:bg-amber-800 disabled:opacity-60"
    >
      {pending ? "Memproses…" : children}
    </button>
  );
}
