import { authedRequest } from "./api";
import type {
  CountItem,
  MovementRequest,
  MovementType,
  StockLevel,
  StockMovement,
} from "./stock";

export interface LevelPage {
  items: StockLevel[];
  next_cursor: string | null;
}

export interface MovementPage {
  items: StockMovement[];
  next_cursor: string | null;
}

export interface CountResult {
  variant_id: string;
  before: string;
  counted: string;
  delta: string;
}

function query(params: Record<string, string | undefined>): string {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) qs.set(k, v);
  return `?${qs}`;
}

export function listStockLevels(
  storeId: string,
  opts: { q?: string; cursor?: string } = {},
): Promise<LevelPage> {
  return authedRequest<LevelPage>(
    `/v1/inventory/levels${query({ store_id: storeId, q: opts.q, cursor: opts.cursor })}`,
  );
}

export function listStockMovements(
  storeId: string,
  opts: { type?: MovementType; cursor?: string } = {},
): Promise<MovementPage> {
  return authedRequest<MovementPage>(
    `/v1/inventory/movements${query({ store_id: storeId, type: opts.type, cursor: opts.cursor })}`,
  );
}

export function createStockMovement(
  req: MovementRequest,
): Promise<{ movement: StockMovement; qty_on_hand: string }> {
  return authedRequest("/v1/inventory/movements", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export function submitStockCounts(
  storeId: string,
  items: CountItem[],
): Promise<{ results: CountResult[]; adjusted: number }> {
  return authedRequest("/v1/inventory/counts", {
    method: "POST",
    body: JSON.stringify({ store_id: storeId, items }),
  });
}
