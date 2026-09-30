"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  createCategory,
  deleteCategory,
  listCategories,
  updateCategory,
} from "@/lib/catalog-api";
import { type Category, buildTree, validateCategoryName } from "@/lib/category";

type Load =
  | { kind: "loading" }
  | { kind: "ready"; items: Category[] }
  | { kind: "error"; message: string };

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";

export default function CategoriesPage() {
  const { profile } = useSession();
  const canWrite =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [actionError, setActionError] = useState<string>();

  const refresh = useCallback(async () => {
    try {
      setLoad({ kind: "ready", items: await listCategories() });
    } catch (e) {
      setLoad({ kind: "error", message: message(e) });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const tree = useMemo(
    () => (load.kind === "ready" ? buildTree(load.items) : []),
    [load],
  );

  async function run(action: () => Promise<unknown>): Promise<boolean> {
    setActionError(undefined);
    try {
      await action();
      await refresh();
      return true;
    } catch (e) {
      setActionError(message(e));
      return false;
    }
  }

  if (load.kind === "loading") return <p className="text-stone-600">Memuat…</p>;
  if (load.kind === "error") {
    return (
      <p role="alert" className="text-red-800">
        {load.message}
      </p>
    );
  }

  return (
    <section className="space-y-4">
      <h1 className="text-xl font-semibold">Kategori</h1>
      {actionError && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
        >
          {actionError}
        </p>
      )}
      {canWrite && (
        <AddForm
          label="Kategori baru"
          onAdd={(name) => run(() => createCategory({ name, parent_id: null }))}
        />
      )}
      {tree.length === 0 ? (
        <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
          Belum ada kategori.
          {canWrite ? " Tambahkan yang pertama di atas." : ""}
        </p>
      ) : (
        <ul className="space-y-3">
          {tree.map((node) => (
            <li
              key={node.id}
              className="rounded-xl border border-stone-200 bg-white p-3"
            >
              <Row category={node} canWrite={canWrite} run={run} strong />
              <ul className="mt-2 space-y-2 border-l-2 border-stone-200 pl-3">
                {node.children.map((child) => (
                  <li key={child.id}>
                    <Row category={child} canWrite={canWrite} run={run} />
                  </li>
                ))}
              </ul>
              {canWrite && (
                <div className="mt-2 pl-3">
                  <AddForm
                    label={`Sub-kategori di ${node.name}`}
                    onAdd={(name) =>
                      run(() => createCategory({ name, parent_id: node.id }))
                    }
                  />
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function AddForm({
  label,
  onAdd,
}: {
  label: string;
  onAdd: (name: string) => Promise<boolean>;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const invalid = validateCategoryName(name);
    setError(invalid);
    if (invalid) return;
    setPending(true);
    if (await onAdd(name)) setName("");
    setPending(false);
  }

  return (
    <form onSubmit={submit} className="flex flex-wrap items-start gap-2">
      <div className="min-w-48 flex-1">
        <input
          aria-label={label}
          placeholder={label}
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-invalid={error ? true : undefined}
          className="min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30"
        />
        {error && <p className="mt-1 text-sm text-red-700">{error}</p>}
      </div>
      <button
        disabled={pending}
        className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800 disabled:opacity-60"
      >
        Tambah
      </button>
    </form>
  );
}

function Row({
  category,
  canWrite,
  run,
  strong,
}: {
  category: Category;
  canWrite: boolean;
  run: (a: () => Promise<unknown>) => Promise<boolean>;
  strong?: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(category.name);
  const [error, setError] = useState<string>();

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const invalid = validateCategoryName(name);
    setError(invalid);
    if (invalid) return;
    if (
      await run(() =>
        updateCategory(category.id, { name, sort_order: category.sort_order }),
      )
    )
      setEditing(false);
  }

  if (editing) {
    return (
      <form onSubmit={save} className="flex flex-wrap items-start gap-2">
        <div className="min-w-48 flex-1">
          <input
            aria-label={`Nama ${category.name}`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base outline-none focus:border-amber-600 focus:ring-2 focus:ring-amber-600/30"
          />
          {error && <p className="mt-1 text-sm text-red-700">{error}</p>}
        </div>
        <button className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white">
          Simpan
        </button>
        <button
          type="button"
          onClick={() => {
            setEditing(false);
            setName(category.name);
            setError(undefined);
          }}
          className="min-h-11 rounded-lg border border-stone-300 px-4 text-sm"
        >
          Batal
        </button>
      </form>
    );
  }

  return (
    <div className="flex items-center justify-between gap-2">
      <span className={strong ? "font-medium" : ""}>{category.name}</span>
      {canWrite && (
        <span className="flex gap-2">
          <button
            onClick={() => setEditing(true)}
            className="min-h-11 rounded-lg border border-stone-300 px-3 text-sm hover:bg-stone-100"
          >
            Ubah
          </button>
          <button
            onClick={() =>
              window.confirm(`Hapus kategori "${category.name}"?`) &&
              void run(() => deleteCategory(category.id))
            }
            className="min-h-11 rounded-lg border border-red-300 px-3 text-sm text-red-800 hover:bg-red-50"
          >
            Hapus
          </button>
        </span>
      )}
    </div>
  );
}
