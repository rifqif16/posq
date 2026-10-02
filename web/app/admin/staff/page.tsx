"use client";

import { useCallback, useEffect, useState } from "react";
import { StaffFormView } from "@/components/StaffForm";
import { StaffSecretForm } from "@/components/StaffSecretForm";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  clearStaffPin,
  createStaff,
  listStaff,
  listStores,
  resetStaffPassword,
  setStaffPin,
  updateStaff,
} from "@/lib/staff-api";
import {
  type StaffForm,
  type StaffMember,
  type StaffRole,
  type StoreInfo,
  emptyStaffForm,
  formFromMember,
  roleLabel,
  storeNames,
  toCreateRequest,
  toUpdateRequest,
  validatePin,
} from "@/lib/staff";
import { validatePassword } from "@/lib/validate";

type Panel =
  | { kind: "create" }
  | { kind: "edit" | "password" | "pin"; id: string }
  | null;

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";
const smallButton =
  "min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-sm hover:bg-stone-100";

export default function StaffPage() {
  const { profile } = useSession();
  const canManage =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [members, setMembers] = useState<StaffMember[]>([]);
  const [roles, setRoles] = useState<StaffRole[]>([]);
  const [stores, setStores] = useState<StoreInfo[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [panel, setPanel] = useState<Panel>(null);

  const reload = useCallback(async () => {
    const [list, storeList] = await Promise.all([listStaff(), listStores()]);
    setMembers(list.items);
    setRoles(list.assignable_roles);
    setStores(storeList);
  }, []);

  useEffect(() => {
    if (!canManage) return;
    reload()
      .then(() => setState("ready"))
      .catch((e) => {
        setError(message(e));
        setState("error");
      });
  }, [canManage, reload]);

  const fail = (e: unknown) => ({
    fields: e instanceof ApiError ? e.fields : {},
    message:
      e instanceof ApiError && Object.keys(e.fields).length > 0
        ? undefined
        : message(e),
  });

  async function create(f: StaffForm) {
    try {
      await createStaff(toCreateRequest(f));
      await reload();
      setPanel(null);
      setNotice(`Akun ${f.name.trim()} dibuat`);
      return null;
    } catch (e) {
      return fail(e);
    }
  }

  async function edit(id: string, f: StaffForm) {
    try {
      await updateStaff(id, toUpdateRequest(f));
      await reload();
      setPanel(null);
      setNotice("Perubahan tersimpan");
      return null;
    } catch (e) {
      return fail(e);
    }
  }

  async function secret(
    action: () => Promise<void>,
    done: string,
  ): Promise<string | null> {
    try {
      await action();
      await reload();
      setPanel(null);
      setNotice(done);
      return null;
    } catch (e) {
      return message(e);
    }
  }

  if (!canManage)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengelola staf.
      </p>
    );

  const defaultRole: StaffRole = roles[0] ?? "cashier";

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Staf</h1>
        {roles.length > 0 && (
          <button
            onClick={() => setPanel({ kind: "create" })}
            className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800"
          >
            Tambah staf
          </button>
        )}
      </div>

      {notice && (
        <p
          role="status"
          className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-900"
        >
          {notice}
        </p>
      )}
      {error && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
        >
          {error}
        </p>
      )}
      {state === "loading" && <p className="text-stone-600">Memuat…</p>}

      {panel?.kind === "create" && (
        <StaffFormView
          mode="create"
          initial={emptyStaffForm(
            defaultRole,
            stores.slice(0, 1).map((s) => s.id),
          )}
          stores={stores}
          roles={roles}
          submitLabel="Buat akun"
          onSubmit={create}
          onCancel={() => setPanel(null)}
        />
      )}

      {state === "ready" && (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {members.map((m) => (
            <li key={m.id} className="space-y-2 p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate font-medium">
                    {m.name}
                    <span className="ml-2 rounded bg-stone-200 px-1.5 py-0.5 text-xs font-normal">
                      {roleLabel(m.role)}
                    </span>
                    {m.status === "disabled" && (
                      <span className="ml-2 rounded bg-red-100 px-1.5 py-0.5 text-xs font-normal text-red-800">
                        Nonaktif
                      </span>
                    )}
                  </p>
                  <p className="truncate text-sm text-stone-600">
                    {m.email} · {storeNames(m.store_ids, stores)}
                    {m.has_pin ? " · PIN aktif" : ""}
                  </p>
                </div>
                {m.manageable && (
                  <div className="flex flex-wrap gap-2">
                    <button
                      className={smallButton}
                      onClick={() => setPanel({ kind: "edit", id: m.id })}
                    >
                      Ubah
                    </button>
                    <button
                      className={smallButton}
                      onClick={() => setPanel({ kind: "password", id: m.id })}
                    >
                      Password
                    </button>
                    <button
                      className={smallButton}
                      onClick={() => setPanel({ kind: "pin", id: m.id })}
                    >
                      PIN
                    </button>
                  </div>
                )}
              </div>
              {panel && panel.kind === "edit" && panel.id === m.id && (
                <StaffFormView
                  mode="edit"
                  initial={formFromMember(m)}
                  stores={stores}
                  roles={roles}
                  submitLabel="Simpan perubahan"
                  onSubmit={(f) => edit(m.id, f)}
                  onCancel={() => setPanel(null)}
                />
              )}
              {panel && panel.kind === "password" && panel.id === m.id && (
                <StaffSecretForm
                  id={`pw-${m.id}`}
                  label="Password baru (min. 10 karakter)"
                  validate={validatePassword}
                  onSubmit={(v) =>
                    secret(
                      () => resetStaffPassword(m.id, v),
                      `Password ${m.name} diganti; sesi lamanya dikeluarkan`,
                    )
                  }
                  onCancel={() => setPanel(null)}
                />
              )}
              {panel && panel.kind === "pin" && panel.id === m.id && (
                <StaffSecretForm
                  id={`pin-${m.id}`}
                  label="PIN baru (6 digit)"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  validate={validatePin}
                  onSubmit={(v) =>
                    secret(() => setStaffPin(m.id, v), `PIN ${m.name} disimpan`)
                  }
                  onCancel={() => setPanel(null)}
                  extra={
                    m.has_pin ? (
                      <button
                        type="button"
                        className={smallButton}
                        onClick={() =>
                          secret(
                            () => clearStaffPin(m.id),
                            `PIN ${m.name} dihapus`,
                          ).then((f) => f && setError(f))
                        }
                      >
                        Hapus PIN
                      </button>
                    ) : undefined
                  }
                />
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
