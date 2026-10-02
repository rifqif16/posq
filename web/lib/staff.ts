import { toggleId } from "./product";
import {
  type FieldErrors,
  MAX_NAME_LEN,
  validateEmail,
  validatePassword,
} from "./validate";

export type StaffRole = "admin" | "cashier" | "kitchen";
export type StaffStatus = "active" | "disabled";

export interface StaffMember {
  id: string;
  name: string;
  email: string;
  role: StaffRole | "owner";
  status: StaffStatus;
  store_ids: string[];
  has_pin: boolean;
  manageable: boolean;
  created_at: string;
}

export interface StoreInfo {
  id: string;
  code: string;
  name: string;
}

export interface StaffForm {
  name: string;
  email: string;
  password: string;
  role: StaffRole;
  store_ids: string[];
  pin: string;
  active: boolean;
}

export interface CreateStaffRequest {
  name: string;
  email: string;
  password: string;
  role: StaffRole;
  store_ids: string[];
  pin?: string;
}

export interface UpdateStaffRequest {
  name: string;
  role: StaffRole;
  store_ids: string[];
  status: StaffStatus;
}

export const MAX_STAFF_STORES = 20;

const ROLE_LABELS: Record<StaffRole | "owner", string> = {
  owner: "Pemilik",
  admin: "Admin",
  cashier: "Kasir",
  kitchen: "Dapur",
};

const WEAK_PINS = new Set(["123456", "654321", "012345", "123123"]);

export function roleLabel(role: StaffRole | "owner"): string {
  return ROLE_LABELS[role];
}

export function emptyStaffForm(role: StaffRole, storeIds: string[]): StaffForm {
  return {
    name: "",
    email: "",
    password: "",
    role,
    store_ids: storeIds,
    pin: "",
    active: true,
  };
}

export function formFromMember(m: StaffMember): StaffForm {
  return {
    name: m.name,
    email: m.email,
    password: "",
    role: m.role === "owner" ? "admin" : m.role,
    store_ids: [...m.store_ids],
    pin: "",
    active: m.status === "active",
  };
}

export function validatePin(pin: string): string | undefined {
  if (!/^\d{6}$/.test(pin)) return "PIN harus 6 digit angka";
  if (WEAK_PINS.has(pin) || /^(\d)\1{5}$/.test(pin))
    return "PIN terlalu mudah ditebak";
}

export function validateStaffForm(
  f: StaffForm,
  mode: "create" | "edit",
): FieldErrors {
  const errors: FieldErrors = {};
  const name = f.name.trim();
  if (name === "" || [...name].length > MAX_NAME_LEN)
    errors.name = "Nama wajib diisi (maks 100 karakter)";
  if (mode === "create") {
    const emailError = validateEmail(f.email);
    if (emailError) errors.email = emailError;
    const passwordError = validatePassword(f.password);
    if (passwordError) errors.password = passwordError;
    if (f.pin !== "") {
      const pinError = validatePin(f.pin);
      if (pinError) errors.pin = pinError;
    }
  }
  if (f.store_ids.length < 1) errors.store_ids = "Pilih minimal satu outlet";
  if (f.store_ids.length > MAX_STAFF_STORES)
    errors.store_ids = `Maksimal ${MAX_STAFF_STORES} outlet`;
  return errors;
}

export function toCreateRequest(f: StaffForm): CreateStaffRequest {
  const req: CreateStaffRequest = {
    name: f.name.trim(),
    email: f.email.trim().toLowerCase(),
    password: f.password,
    role: f.role,
    store_ids: f.store_ids,
  };
  if (f.pin !== "") req.pin = f.pin;
  return req;
}

export function toUpdateRequest(f: StaffForm): UpdateStaffRequest {
  return {
    name: f.name.trim(),
    role: f.role,
    store_ids: f.store_ids,
    status: f.active ? "active" : "disabled",
  };
}

export function toggleStore(ids: string[], id: string): string[] {
  return toggleId(ids, id);
}

export function storeNames(ids: string[], stores: StoreInfo[]): string {
  const byId = new Map(stores.map((s) => [s.id, s.name]));
  return ids.map((id) => byId.get(id) ?? "Outlet tidak dikenal").join(", ");
}
