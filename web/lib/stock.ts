import { parseRupiah } from "./money";
import type { FieldErrors } from "./validate";

export type MovementType =
  | "purchase_receive"
  | "waste"
  | "adjustment"
  | "opname";
export type MovementKind = "receive" | "waste" | "adjust_in" | "adjust_out";

export interface StockLevel {
  variant_id: string;
  product_id: string;
  product_name: string;
  variant_name: string;
  sku: string;
  qty_on_hand: string;
}

export interface StockMovement {
  id: string;
  variant_id: string;
  product_name: string;
  variant_name: string;
  sku: string;
  type: MovementType;
  qty_delta: string;
  unit_cost: number | null;
  reason: string;
  created_by: { id: string; name: string };
  created_at: string;
}

export interface MovementForm {
  kind: MovementKind;
  qty: string;
  reason: string;
  unit_cost: string;
}

export interface MovementRequest {
  store_id: string;
  variant_id: string;
  type: "purchase_receive" | "waste" | "adjustment";
  qty_delta: string;
  unit_cost?: number;
  reason?: string;
}

export interface CountItem {
  variant_id: string;
  counted_qty: string;
}

const QTY = /^\d{1,9}([.,]\d{1,3})?$/;

export const KIND_LABELS: Record<MovementKind, string> = {
  receive: "Stok masuk",
  waste: "Rusak / kedaluwarsa / dipakai sendiri",
  adjust_in: "Koreksi tambah",
  adjust_out: "Koreksi kurang",
};

export function emptyMovementForm(
  kind: MovementKind = "receive",
): MovementForm {
  return { kind, qty: "", reason: "", unit_cost: "" };
}

export function normalizeQty(raw: string): string | null {
  const t = raw.trim();
  return QTY.test(t) ? t.replace(",", ".") : null;
}

export function displayQty(value: string): string {
  const n = Number(value);
  if (Number.isNaN(n)) return value;
  return n.toLocaleString("id-ID", { maximumFractionDigits: 3 });
}

export function formatSigned(delta: string): string {
  const n = Number(delta);
  return `${n > 0 ? "+" : ""}${displayQty(delta)}`;
}

export function isNegative(value: string): boolean {
  return Number(value) < 0;
}

export function itemLabel(x: {
  product_name: string;
  variant_name: string;
}): string {
  return x.variant_name === "Default"
    ? x.product_name
    : `${x.product_name} · ${x.variant_name}`;
}

export function movementTypeLabel(type: MovementType): string {
  switch (type) {
    case "purchase_receive":
      return "Stok masuk";
    case "waste":
      return "Rusak / keluar";
    case "adjustment":
      return "Koreksi";
    case "opname":
      return "Opname";
  }
}

const needsReason = (kind: MovementKind) => kind !== "receive";

export function validateMovementForm(f: MovementForm): FieldErrors {
  const errors: FieldErrors = {};
  const qty = normalizeQty(f.qty);
  if (qty === null) errors.qty = "Jumlah harus angka dengan maksimal 3 desimal";
  else if (Number(qty) === 0) errors.qty = "Jumlah tidak boleh nol";
  if (needsReason(f.kind) && f.reason.trim() === "")
    errors.reason = "Alasan wajib diisi";
  if (f.reason.trim().length > 200)
    errors.reason = "Alasan maksimal 200 karakter";
  if (f.kind === "receive" && f.unit_cost.trim() !== "") {
    const cost = parseRupiah(f.unit_cost);
    if (cost === null || cost > 1_000_000_000)
      errors.unit_cost = "Harga satuan harus angka antara 0 dan 1.000.000.000";
  }
  return errors;
}

export function toMovementRequest(
  storeId: string,
  variantId: string,
  f: MovementForm,
): MovementRequest {
  const qty = normalizeQty(f.qty) ?? "0";
  const negative = f.kind === "waste" || f.kind === "adjust_out";
  const req: MovementRequest = {
    store_id: storeId,
    variant_id: variantId,
    type:
      f.kind === "receive"
        ? "purchase_receive"
        : f.kind === "waste"
          ? "waste"
          : "adjustment",
    qty_delta: negative ? `-${qty}` : qty,
  };
  if (f.reason.trim() !== "") req.reason = f.reason.trim();
  if (f.kind === "receive" && f.unit_cost.trim() !== "")
    req.unit_cost = parseRupiah(f.unit_cost) ?? 0;
  return req;
}

export function collectCounts(inputs: Record<string, string>): {
  items: CountItem[];
  errors: Record<string, string>;
} {
  const items: CountItem[] = [];
  const errors: Record<string, string> = {};
  for (const [variantId, raw] of Object.entries(inputs)) {
    if (raw.trim() === "") continue;
    const qty = normalizeQty(raw);
    if (qty === null)
      errors[variantId] = "Angka nol atau lebih, maksimal 3 desimal";
    else items.push({ variant_id: variantId, counted_qty: qty });
  }
  return { items, errors };
}
