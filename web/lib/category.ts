export const MAX_CATEGORY_NAME_LEN = 100;

export interface Category {
  id: string;
  parent_id: string | null;
  name: string;
  sort_order: number;
}

export interface CategoryNode extends Category {
  children: Category[];
}

export function validateCategoryName(raw: string): string | undefined {
  const name = raw.trim();
  if (name === "" || [...name].length > MAX_CATEGORY_NAME_LEN) {
    return `Nama kategori wajib diisi (maks ${MAX_CATEGORY_NAME_LEN} karakter)`;
  }
}

export function buildTree(items: Category[]): CategoryNode[] {
  const roots: CategoryNode[] = items
    .filter((c) => c.parent_id === null)
    .map((c) => ({ ...c, children: [] }));
  const byId = new Map(roots.map((r) => [r.id, r]));
  for (const c of items) {
    if (c.parent_id !== null) byId.get(c.parent_id)?.children.push(c);
  }
  return roots;
}
