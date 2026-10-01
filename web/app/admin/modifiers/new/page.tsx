"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { ModifierGroupFormView } from "@/components/ModifierGroupForm";
import type { SubmitError } from "@/components/ProductForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import { createModifierGroup } from "@/lib/catalog-api";
import { type ModifierGroupRequest, emptyGroupForm } from "@/lib/modifier";

export default function NewModifierGroupPage() {
  const router = useRouter();
  const { profile } = useSession();
  const canManage =
    profile.user.role === "owner" || profile.user.role === "admin";

  async function onSubmit(
    req: ModifierGroupRequest,
  ): Promise<SubmitError | null> {
    try {
      await createModifierGroup(req);
      router.push("/admin/modifiers");
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

  if (!canManage)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengelola modifier.
      </p>
    );

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Grup modifier baru</h1>
        <Link
          href="/admin/modifiers"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>
      <ModifierGroupFormView
        initial={emptyGroupForm()}
        submitLabel="Simpan grup"
        onSubmit={onSubmit}
      />
    </section>
  );
}
