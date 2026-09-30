import { authedRequest } from "./api";
import type { Category } from "./category";

export async function listCategories(): Promise<Category[]> {
  const res = await authedRequest<{ items: Category[] }>("/v1/categories");
  return res.items;
}

export function createCategory(input: {
  name: string;
  parent_id: string | null;
  sort_order?: number;
}): Promise<Category> {
  return authedRequest<Category>("/v1/categories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCategory(
  id: string,
  input: { name: string; sort_order: number },
): Promise<Category> {
  return authedRequest<Category>(`/v1/categories/${id}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteCategory(id: string): Promise<void> {
  return authedRequest<void>(`/v1/categories/${id}`, { method: "DELETE" });
}
