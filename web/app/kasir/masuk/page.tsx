"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { PinPad } from "@/components/PinPad";
import { type PinUser, ApiError, fetchPinUsers, pinLogin } from "@/lib/api";
import { type StoredDevice, clearDevice, loadDevice } from "@/lib/device";

type State =
  | { kind: "loading" }
  | { kind: "unregistered" }
  | { kind: "invalid-device" }
  | {
      kind: "ready";
      device: StoredDevice;
      deviceName: string;
      users: PinUser[];
    }
  | { kind: "error"; message: string };

export default function CashierLoginPage() {
  const router = useRouter();
  const [state, setState] = useState<State>({ kind: "loading" });
  const [selected, setSelected] = useState<PinUser | null>(null);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    const device = loadDevice(window.localStorage);
    if (!device) {
      setState({ kind: "unregistered" });
      return;
    }
    try {
      const res = await fetchPinUsers(device);
      setState({
        kind: "ready",
        device,
        deviceName: res.device_name,
        users: res.items,
      });
    } catch (e) {
      if (e instanceof ApiError && e.status === 401)
        setState({ kind: "invalid-device" });
      else
        setState({
          kind: "error",
          message:
            e instanceof ApiError
              ? e.message
              : "Tidak dapat terhubung ke server",
        });
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function submitPin(device: StoredDevice, pin: string) {
    if (!selected) return;
    setError(undefined);
    try {
      await pinLogin(device, selected.id, pin);
      router.replace("/kasir");
    } catch (e) {
      setError(
        e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server",
      );
    }
  }

  function forgetDevice() {
    clearDevice(window.localStorage);
    setSelected(null);
    setState({ kind: "unregistered" });
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 px-4 py-6">
      <header>
        <h1 className="text-2xl font-semibold">Masuk kasir</h1>
        {state.kind === "ready" && (
          <p className="text-sm text-stone-600">
            Perangkat: {state.deviceName}
          </p>
        )}
      </header>

      {state.kind === "loading" && <p className="text-stone-600">Memuat…</p>}

      {state.kind === "unregistered" && (
        <div className="space-y-3">
          <p className="text-stone-700">
            Perangkat ini belum didaftarkan. Minta Owner atau Admin
            mendaftarkannya di menu Admin › Perangkat.
          </p>
          <Link
            href="/login"
            className="text-sm font-medium text-amber-800 underline"
          >
            Masuk dengan email
          </Link>
        </div>
      )}

      {state.kind === "invalid-device" && (
        <div className="space-y-3">
          <p role="alert" className="text-red-800">
            Perangkat ini tidak valid atau sudah dicabut.
          </p>
          <button
            onClick={forgetDevice}
            className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
          >
            Lepas perangkat ini
          </button>
        </div>
      )}

      {state.kind === "error" && (
        <div className="space-y-3">
          <p role="alert" className="text-red-800">
            {state.message}
          </p>
          <button
            onClick={() => void load()}
            className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
          >
            Coba lagi
          </button>
        </div>
      )}

      {state.kind === "ready" &&
        !selected &&
        (state.users.length === 0 ? (
          <p className="text-stone-700">
            Belum ada kasir ber-PIN di outlet ini. Atur PIN di menu Admin ›
            Staf.
          </p>
        ) : (
          <ul className="space-y-2">
            {state.users.map((u) => (
              <li key={u.id}>
                <button
                  onClick={() => setSelected(u)}
                  className="min-h-14 w-full rounded-xl border border-stone-300 bg-white px-4 text-left text-lg hover:bg-stone-100"
                >
                  {u.name}
                </button>
              </li>
            ))}
          </ul>
        ))}

      {state.kind === "ready" && selected && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-lg font-medium">{selected.name}</p>
            <button
              onClick={() => {
                setSelected(null);
                setError(undefined);
              }}
              className="text-sm text-amber-800 underline"
            >
              Ganti
            </button>
          </div>
          {error && (
            <p
              role="alert"
              className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
            >
              {error}
            </p>
          )}
          <PinPad onSubmit={(pin) => submitPin(state.device, pin)} />
        </div>
      )}
    </main>
  );
}
