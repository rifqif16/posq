// Cermin aturan server (auth/domain/user.go) untuk umpan balik cepat.
// Server tetap sumber kebenaran; jangan tambah aturan baru hanya di sini.
export const MIN_PASSWORD_LEN = 10;
export const MAX_PASSWORD_LEN = 128;
export const MAX_NAME_LEN = 100;

export type FieldErrors = Partial<Record<string, string>>;

export function validateEmail(raw: string): string | undefined {
  const email = raw.trim();
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) || email.length > 254) {
    return "Format email tidak valid";
  }
}

export function validatePassword(p: string): string | undefined {
  if ([...p].length < MIN_PASSWORD_LEN)
    return `Password minimal ${MIN_PASSWORD_LEN} karakter`;
  if (p.length > MAX_PASSWORD_LEN)
    return `Password maksimal ${MAX_PASSWORD_LEN} karakter`;
}

function validateName(v: string, label: string): string | undefined {
  const t = v.trim();
  if (t === "" || [...t].length > MAX_NAME_LEN)
    return `${label} wajib diisi (maks ${MAX_NAME_LEN} karakter)`;
}

export interface RegisterInput {
  business_name: string;
  owner_name: string;
  email: string;
  password: string;
}

export function validateRegister(input: RegisterInput): FieldErrors {
  const errors: FieldErrors = {
    business_name: validateName(input.business_name, "Nama usaha"),
    owner_name: validateName(input.owner_name, "Nama pemilik"),
    email: validateEmail(input.email),
    password: validatePassword(input.password),
  };
  return Object.fromEntries(
    Object.entries(errors).filter(([, v]) => v !== undefined),
  );
}
