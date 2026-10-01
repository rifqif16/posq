import { formatRupiah, parseRupiah } from "./money";
import { MAX_PRICE } from "./product";
import type { FieldErrors } from "./validate";

export const MAX_MODIFIERS = 50; // opsi per grup; juga batas max_select

export interface Modifier {
  id: string;
  name: string;
  price_delta: number;
  is_default: boolean;
  is_active: boolean;
}

export interface ModifierGroup {
  id: string;
  name: string;
  min_select: number;
  max_select: number;
  is_required: boolean;
  version: number;
  modifiers: Modifier[];
}

export interface ModifierForm {
  key: string; // kunci React lokal, tidak dikirim ke server
  id?: string; // ada bila opsi sudah tersimpan
  name: string;
  price_delta: string;
  is_default: boolean;
  is_active: boolean;
}

export interface ModifierGroupForm {
  name: string;
  min_select: string;
  max_select: string;
  modifiers: ModifierForm[];
}

export interface ModifierGroupRequest {
  name: string;
  min_select: number;
  max_select: number;
  modifiers: {
    id?: string;
    name: string;
    price_delta: number;
    is_default: boolean;
    is_active: boolean;
  }[];
}

let keySeq = 0;
const newKey = () => `m${++keySeq}`;

export function emptyModifier(): ModifierForm {
  return {
    key: newKey(),
    name: "",
    price_delta: "",
    is_default: false,
    is_active: true,
  };
}

export function emptyGroupForm(): ModifierGroupForm {
  return {
    name: "",
    min_select: "0",
    max_select: "1",
    modifiers: [emptyModifier(), emptyModifier()],
  };
}

export function formFromGroup(g: ModifierGroup): ModifierGroupForm {
  return {
    name: g.name,
    min_select: String(g.min_select),
    max_select: String(g.max_select),
    modifiers: g.modifiers.map((m) => ({
      key: m.id,
      id: m.id,
      name: m.name,
      price_delta: m.price_delta === 0 ? "" : String(m.price_delta),
      is_default: m.is_default,
      is_active: m.is_active,
    })),
  };
}

export function parseCount(raw: string): number | null {
  const t = raw.trim();
  return /^\d{1,6}$/.test(t) ? Number(t) : null;
}

export function validateGroupForm(f: ModifierGroupForm): FieldErrors {
  const errors: FieldErrors = {};
  const add = (key: string, msg: string | undefined) => {
    if (msg && !errors[key]) errors[key] = msg;
  };

  const name = f.name.trim();
  add(
    "name",
    name === "" || [...name].length > 100
      ? "Nama grup wajib diisi (maks 100 karakter)"
      : undefined,
  );

  const min = parseCount(f.min_select);
  const max = parseCount(f.max_select);
  add(
    "min_select",
    min === null || min > MAX_MODIFIERS
      ? `Minimal pilihan harus angka 0–${MAX_MODIFIERS}`
      : undefined,
  );
  if (max === null || max < 1 || max > MAX_MODIFIERS)
    add("max_select", `Maksimal pilihan harus angka 1–${MAX_MODIFIERS}`);
  else if (min !== null && max < min)
    add("max_select", "Maksimal pilihan tidak boleh kurang dari minimal");

  if (f.modifiers.length < 1 || f.modifiers.length > MAX_MODIFIERS)
    add("modifiers", `Grup harus memiliki 1 sampai ${MAX_MODIFIERS} opsi`);

  const names = new Set<string>();
  f.modifiers.forEach((m, i) => {
    const p = `modifiers[${i}]`;
    const n = m.name.trim();
    if (n === "" || [...n].length > 100)
      add(`${p}.name`, "Nama opsi wajib diisi (maks 100 karakter)");
    else if (names.has(n.toLowerCase()))
      add(`${p}.name`, "Nama opsi harus unik dalam grup");
    names.add(n.toLowerCase());

    if (m.price_delta.trim() !== "") {
      const price = parseRupiah(m.price_delta);
      if (price === null || price > MAX_PRICE)
        add(
          `${p}.price_delta`,
          `Tambahan harga harus angka antara 0 dan ${MAX_PRICE.toLocaleString("id-ID")}`,
        );
    }
    if (m.is_default && !m.is_active)
      add(`${p}.is_default`, "Opsi default harus aktif");
  });

  const active = f.modifiers.filter((m) => m.is_active).length;
  const defaults = f.modifiers.filter((m) => m.is_default).length;
  if (min !== null && min > active)
    add("min_select", "Minimal pilihan melebihi jumlah opsi aktif");
  if (max !== null && max >= 1 && defaults > max)
    add("modifiers", "Jumlah opsi default melebihi maksimal pilihan");
  return errors;
}

export function toGroupRequest(f: ModifierGroupForm): ModifierGroupRequest {
  return {
    name: f.name.trim(),
    min_select: parseCount(f.min_select) ?? 0,
    max_select: parseCount(f.max_select) ?? 1,
    modifiers: f.modifiers.map((m) => ({
      ...(m.id ? { id: m.id } : {}),
      name: m.name.trim(),
      price_delta: parseRupiah(m.price_delta) ?? 0,
      is_default: m.is_default,
      is_active: m.is_active,
    })),
  };
}

export function moveItem<T>(items: T[], from: number, to: number): T[] {
  if (
    from === to ||
    from < 0 ||
    to < 0 ||
    from >= items.length ||
    to >= items.length
  )
    return items;
  const next = items.slice();
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}

export function selectionLabel(min: number, max: number): string {
  if (min === 0 && max === 1) return "Pilih 1 (opsional)";
  if (min === 1 && max === 1) return "Pilih 1 (wajib)";
  if (min === max) return `Pilih tepat ${max}`;
  if (min === 0) return `Pilih hingga ${max}`;
  return `Pilih ${min}–${max}`;
}

export function priceDeltaLabel(delta: number): string {
  return delta === 0 ? "Gratis" : `+${formatRupiah(delta)}`;
}
