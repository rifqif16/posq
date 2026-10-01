import { formatRupiah } from "./money";

export interface PriceChange {
  id: string;
  variant_id: string;
  variant_name: string;
  field: "cost_price" | "sell_price";
  old_value: number;
  new_value: number;
  changed_by: { id: string; name: string };
  at: string;
}

export interface PriceChangeView {
  fieldLabel: string;
  from: string;
  to: string;
  direction: "naik" | "turun";
  difference: string;
}

export function fieldLabel(field: PriceChange["field"]): string {
  return field === "sell_price" ? "Harga jual" : "Harga beli";
}

export function describeChange(c: PriceChange): PriceChangeView {
  const delta = c.new_value - c.old_value;
  return {
    fieldLabel: fieldLabel(c.field),
    from: formatRupiah(c.old_value),
    to: formatRupiah(c.new_value),
    direction: delta >= 0 ? "naik" : "turun",
    difference: `${delta >= 0 ? "+" : "-"}${formatRupiah(Math.abs(delta))}`,
  };
}

export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}

export function showVariantName(changes: PriceChange[]): boolean {
  return (
    new Set(changes.map((c) => c.variant_id)).size > 1 ||
    changes.some((c) => c.variant_name !== "Default")
  );
}
