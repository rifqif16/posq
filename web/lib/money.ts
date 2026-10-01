export function formatRupiah(n: number): string {
  return `Rp ${Math.trunc(n).toLocaleString("id-ID")}`;
}

export function parseRupiah(raw: string): number | null {
  const digits = raw.replace(/\D/g, "");
  if (digits === "") return null;
  const n = Number(digits);
  return Number.isSafeInteger(n) ? n : null;
}
