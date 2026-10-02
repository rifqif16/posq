"use client";

import { useCallback, useEffect, useState } from "react";
import { Field, FormError } from "@/components/Field";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  type Device,
  type StoredDevice,
  deviceStatusLabel,
  loadDevice,
  saveDevice,
  validateDeviceName,
} from "@/lib/device";
import {
  type RegisteredDevice,
  listDevices,
  registerDevice,
  revokeDevice,
} from "@/lib/device-api";
import { formatDateTime } from "@/lib/price-history";
import { listStores } from "@/lib/staff-api";
import type { StoreInfo } from "@/lib/staff";

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";
const smallButton =
  "min-h-11 rounded-lg border border-stone-300 bg-white px-3 text-sm hover:bg-stone-100";

export default function DevicesPage() {
  const { profile } = useSession();
  const canManage =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [devices, setDevices] = useState<Device[]>([]);
  const [stores, setStores] = useState<StoreInfo[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [name, setName] = useState("");
  const [storeId, setStoreId] = useState("");
  const [nameError, setNameError] = useState<string>();
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);
  const [fresh, setFresh] = useState<RegisteredDevice | null>(null);
  const [thisDevice, setThisDevice] = useState<StoredDevice | null>(null);

  const reload = useCallback(async () => {
    const [list, storeList] = await Promise.all([listDevices(), listStores()]);
    setDevices(list);
    setStores(storeList);
    setStoreId((current) => current || storeList[0]?.id || "");
  }, []);

  useEffect(() => {
    setThisDevice(loadDevice(window.localStorage));
    if (!canManage) return;
    reload()
      .then(() => setState("ready"))
      .catch((e) => {
        setError(message(e));
        setState("error");
      });
  }, [canManage, reload]);

  async function register(e: React.FormEvent) {
    e.preventDefault();
    const invalid = validateDeviceName(name);
    setNameError(invalid);
    setFormError(undefined);
    if (invalid) return;
    setPending(true);
    try {
      const created = await registerDevice(storeId, name.trim());
      setFresh(created);
      setName("");
      await reload();
    } catch (err) {
      if (err instanceof ApiError && err.fields.name)
        setNameError(err.fields.name);
      else setFormError(message(err));
    } finally {
      setPending(false);
    }
  }

  function useHere(d: RegisteredDevice) {
    const stored: StoredDevice = {
      device_id: d.id,
      device_secret: d.secret,
      store_id: d.store_id,
      name: d.name,
    };
    saveDevice(window.localStorage, stored);
    setThisDevice(stored);
    setFresh(null);
    setNotice(
      `Perangkat ini sekarang terdaftar sebagai ${d.name}. Kasir dapat masuk di /kasir/masuk.`,
    );
  }

  async function revoke(d: Device) {
    if (
      !window.confirm(
        `Cabut perangkat "${d.name}"? Perangkat tersebut tidak dapat lagi masuk dengan PIN.`,
      )
    )
      return;
    try {
      await revokeDevice(d.id);
      await reload();
      setNotice(`Perangkat ${d.name} dicabut`);
    } catch (err) {
      setError(message(err));
    }
  }

  if (!canManage)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengelola perangkat.
      </p>
    );

  const storeName = (id: string) =>
    stores.find((s) => s.id === id)?.name ?? "Outlet tidak dikenal";

  return (
    <section className="space-y-4">
      <h1 className="text-xl font-semibold">Perangkat</h1>
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

      {fresh && (
        <div className="space-y-3 rounded-xl border border-amber-300 bg-amber-50 p-4">
          <p className="font-medium">
            Perangkat {fresh.name} ({fresh.code}) terdaftar
          </p>
          <p className="text-sm text-amber-900">
            Kode rahasia hanya ditampilkan sekali. Pasang di perangkat ini
            sekarang, atau catat untuk perangkat lain.
          </p>
          <p className="break-all rounded bg-white px-3 py-2 font-mono text-sm">
            {fresh.secret}
          </p>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => useHere(fresh)}
              className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800"
            >
              Pasang di perangkat ini
            </button>
            <button onClick={() => setFresh(null)} className={smallButton}>
              Tutup
            </button>
          </div>
        </div>
      )}

      <form
        onSubmit={register}
        noValidate
        className="space-y-3 rounded-xl border border-stone-200 bg-white p-4"
      >
        <h2 className="font-medium">Daftarkan perangkat</h2>
        <FormError message={formError} />
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            id="device-name"
            label="Nama perangkat"
            placeholder="mis. Kasir Depan"
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={nameError}
            required
          />
          <div className="space-y-1">
            <label
              htmlFor="device-store"
              className="block text-sm font-medium text-stone-700"
            >
              Outlet
            </label>
            <select
              id="device-store"
              value={storeId}
              onChange={(e) => setStoreId(e.target.value)}
              className="block min-h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-base"
            >
              {stores.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <button
          disabled={pending || storeId === ""}
          className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800 disabled:opacity-60"
        >
          {pending ? "Mendaftarkan…" : "Daftarkan"}
        </button>
      </form>

      {state === "loading" && <p className="text-stone-600">Memuat…</p>}
      {state === "ready" && devices.length === 0 && (
        <p className="rounded-xl border border-dashed border-stone-300 p-6 text-center text-stone-600">
          Belum ada perangkat terdaftar.
        </p>
      )}
      {state === "ready" && devices.length > 0 && (
        <ul className="divide-y divide-stone-200 rounded-xl border border-stone-200 bg-white">
          {devices.map((d) => (
            <li
              key={d.id}
              className="flex flex-wrap items-center justify-between gap-2 p-3"
            >
              <div className="min-w-0">
                <p className="truncate font-medium">
                  {d.name}
                  <span className="ml-2 rounded bg-stone-200 px-1.5 py-0.5 text-xs font-normal">
                    {d.code}
                  </span>
                  {d.status === "revoked" && (
                    <span className="ml-2 rounded bg-red-100 px-1.5 py-0.5 text-xs font-normal text-red-800">
                      {deviceStatusLabel(d.status)}
                    </span>
                  )}
                  {thisDevice?.device_id === d.id && (
                    <span className="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-xs font-normal text-amber-900">
                      Perangkat ini
                    </span>
                  )}
                </p>
                <p className="truncate text-sm text-stone-600">
                  {storeName(d.store_id)} · didaftarkan {d.registered_by}
                  {d.last_seen_at
                    ? ` · terakhir dipakai ${formatDateTime(d.last_seen_at)}`
                    : " · belum pernah dipakai"}
                </p>
              </div>
              {d.status === "active" && (
                <button className={smallButton} onClick={() => void revoke(d)}>
                  Cabut
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
