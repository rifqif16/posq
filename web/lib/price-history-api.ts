import { authedRequest } from "./api";
import type { PriceChange } from "./price-history";

export interface PriceHistoryPage {
  items: PriceChange[];
  next_cursor: string | null;
}

export function listPriceHistory(
  productId: string,
  cursor?: string,
): Promise<PriceHistoryPage> {
  const qs = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return authedRequest<PriceHistoryPage>(
    `/v1/products/${productId}/price-history${qs}`,
  );
}
