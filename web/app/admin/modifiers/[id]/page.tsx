"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ModifierGroupFormView } from "@/components/ModifierGroupForm";
import type { SubmitError } from "@/components/ProductForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  deleteModifierGroup,
  getModifierGroup,
  updateModifierGroup,
} from "@/lib/catalog-api";
import {
  type ModifierGroup,
  type ModifierGroupRequest,
  formFromGroup,
} from "@/lib/modifier";

type Load =
  | { kind: "loading" }
  | { kind: "ready"; group: ModifierGroup }
  | { kind: "error"; message: string };

export default function EditModifierGroupPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { profile } = useSession();
  const canManage =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [load, setLoad] = useState<Load>({ kind: "loading" });

  useEffect(() => {
    if (!canManage) return;
    let cancelled = false;
    getModifierGroup(id)
      .then((group) => !cancelled && setLoad({ kind: "ready", group }))
      .catch((e) => {
        if (cancelled) return;
        const notFound = e instanceof ApiError && e.status === 404;
        setLoad({
          kind: "error",
          message: notFound
            ? "Grup tidak ditemukan"
            : "Tidak dapat memuat grup",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [id, canManage]);

  if (!canManage)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengelola modifier.
      </p>
    );
  if (load.kind === "loading") return <p className="text-stone-600">Memuat…</p>;
  if (load.kind === "error") {
    return (
      <div className="space-y-2">
        <p role="alert" className="text-red-800">
          {load.message}
        </p>
        <Link
          href="/admin/modifiers"
          className="text-sm text-amber-800 underline"
        >
          Kembali ke daftar
        </Link>
      </div>
    );
  }

  const { group } = load;

  async function onSubmit(
    req: ModifierGroupRequest,
  ): Promise<SubmitError | null> {
    try {
      await updateModifierGroup(group.id, group.version, req);
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

  async function onDelete(): Promise<string | null> {
    try {
      await deleteModifierGroup(group.id);
      router.push("/admin/modifiers");
      return null;
    } catch (e) {
      return e instanceof ApiError
        ? e.message
        : "Tidak dapat terhubung ke server";
    }
  }

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Ubah grup modifier</h1>
        <Link
          href="/admin/modifiers"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>
      <ModifierGroupFormView
        initial={formFromGroup(group)}
        submitLabel="Simpan perubahan"
        onSubmit={onSubmit}
        onDelete={onDelete}
      />
    </section>
  );
}
