import { authedDownload, authedRequest } from "./api";
import type { ImportReport } from "./csv-import";

export type CsvDelimiter = "comma" | "semicolon";

function postCsv(path: string, text: string): Promise<ImportReport> {
  return authedRequest<ImportReport>(path, {
    method: "POST",
    headers: { "Content-Type": "text/csv" },
    body: text,
  });
}

export function validateProductCsv(text: string): Promise<ImportReport> {
  return postCsv("/v1/products/import", text);
}

export function commitProductCsv(text: string): Promise<ImportReport> {
  return postCsv("/v1/products/import?commit=true", text);
}

export function downloadProductsCsv(
  delimiter: CsvDelimiter,
): Promise<{ blob: Blob; filename: string }> {
  return authedDownload(`/v1/products/export?delimiter=${delimiter}`);
}
