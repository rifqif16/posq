export const DEVICE_STORAGE_KEY = "posq.device";
export const PIN_LENGTH = 6;

export interface StoredDevice {
  device_id: string;
  device_secret: string;
  store_id: string;
  name: string;
}

export interface KeyValueStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isStoredDevice(value: unknown): value is StoredDevice {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.device_id === "string" &&
    UUID.test(v.device_id) &&
    typeof v.store_id === "string" &&
    UUID.test(v.store_id) &&
    typeof v.device_secret === "string" &&
    v.device_secret.length >= 20 &&
    typeof v.name === "string"
  );
}

export function saveDevice(store: KeyValueStore, device: StoredDevice): void {
  store.setItem(DEVICE_STORAGE_KEY, JSON.stringify(device));
}

export function loadDevice(store: KeyValueStore): StoredDevice | null {
  const raw = store.getItem(DEVICE_STORAGE_KEY);
  if (raw === null) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return isStoredDevice(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function clearDevice(store: KeyValueStore): void {
  store.removeItem(DEVICE_STORAGE_KEY);
}

export function appendPinDigit(current: string, digit: string): string {
  if (!/^\d$/.test(digit) || current.length >= PIN_LENGTH) return current;
  return current + digit;
}

export function removePinDigit(current: string): string {
  return current.slice(0, -1);
}

export interface Device {
  id: string;
  store_id: string;
  code: string;
  name: string;
  status: "active" | "revoked";
  registered_at: string;
  last_seen_at: string | null;
  registered_by: string;
}

export function validateDeviceName(raw: string): string | undefined {
  const name = raw.trim();
  if (name === "" || [...name].length > 60)
    return "Nama perangkat wajib diisi (maks 60 karakter)";
}

export function deviceStatusLabel(status: Device["status"]): string {
  return status === "active" ? "Aktif" : "Dicabut";
}
