"use client";

import Link from "next/link";
import { useState } from "react";
import { useSession } from "@/components/SessionProvider";
import { ApiError } from "@/lib/api";
import {
  type ImportReport,
  describeReport,
  fileProblem,
  hiddenErrorCount,
  locationLabel,
  templateCsv,
} from "@/lib/csv-import";
import {
  type CsvDelimiter,
  commitProductCsv,
  downloadProductsCsv,
  validateProductCsv,
} from "@/lib/product-csv-api";

type State =
  | { kind: "idle" }
  | { kind: "working"; label: string }
  | { kind: "report"; report: ImportReport; text: string; fileName: string }
  | { kind: "done"; report: ImportReport };

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

const message = (e: unknown) =>
  e instanceof ApiError ? e.message : "Tidak dapat terhubung ke server";
const buttonClass =
  "min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100 disabled:opacity-60";

export default function ProductImportPage() {
  const { profile } = useSession();
  const canWrite =
    profile.user.role === "owner" || profile.user.role === "admin";
  const [state, setState] = useState<State>({ kind: "idle" });
  const [error, setError] = useState<string>();
  const [fileKey, setFileKey] = useState(0);

  const busy = state.kind === "working";

  async function exportCsv(delimiter: CsvDelimiter) {
    setError(undefined);
    setState({ kind: "working", label: "Menyiapkan ekspor…" });
    try {
      const { blob, filename } = await downloadProductsCsv(delimiter);
      saveBlob(blob, filename);
    } catch (e) {
      setError(message(e));
    } finally {
      setState({ kind: "idle" });
    }
  }

  function downloadTemplate() {
    saveBlob(
      new Blob(["\uFEFF" + templateCsv()], { type: "text/csv;charset=utf-8" }),
      "template-produk.csv",
    );
  }

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError(undefined);
    const problem = fileProblem(file);
    if (problem) {
      setError(problem);
      setFileKey((k) => k + 1);
      return;
    }
    setState({ kind: "working", label: "Memvalidasi file…" });
    try {
      const text = await file.text();
      setState({
        kind: "report",
        report: await validateProductCsv(text),
        text,
        fileName: file.name,
      });
    } catch (err) {
      setError(message(err));
      setState({ kind: "idle" });
    }
  }

  async function commit(text: string) {
    setError(undefined);
    setState({ kind: "working", label: "Mengimpor produk…" });
    try {
      setState({ kind: "done", report: await commitProductCsv(text) });
    } catch (err) {
      setError(message(err));
      setState({ kind: "idle" });
    }
  }

  function reset() {
    setState({ kind: "idle" });
    setError(undefined);
    setFileKey((k) => k + 1);
  }

  if (!canWrite)
    return (
      <p className="text-stone-700">
        Anda tidak memiliki izin untuk mengimpor atau mengekspor produk.
      </p>
    );

  return (
    <section className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Impor / ekspor produk</h1>
        <Link
          href="/admin/products"
          className="text-sm text-amber-800 underline"
        >
          Kembali
        </Link>
      </div>

      {error && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800"
        >
          {error}
        </p>
      )}

      <div className="space-y-3 rounded-xl border border-stone-200 bg-white p-4">
        <h2 className="font-medium">Ekspor</h2>
        <p className="text-sm text-stone-600">
          Satu baris per varian. Pilih titik koma bila file dibuka dengan Excel
          berbahasa Indonesia.
        </p>
        <div className="flex flex-wrap gap-2">
          <button
            className={buttonClass}
            disabled={busy}
            onClick={() => exportCsv("comma")}
          >
            CSV (koma)
          </button>
          <button
            className={buttonClass}
            disabled={busy}
            onClick={() => exportCsv("semicolon")}
          >
            CSV (titik koma)
          </button>
        </div>
      </div>

      <div className="space-y-3 rounded-xl border border-stone-200 bg-white p-4">
        <h2 className="font-medium">Impor produk baru</h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-stone-600">
          <li>
            Impor hanya menambah produk baru; produk dengan nama, SKU, atau
            barcode yang sudah ada akan ditolak.
          </li>
          <li>
            Kategori dan grup modifier harus sudah dibuat. Kategori anak ditulis{" "}
            <code>Induk &gt; Anak</code>; beberapa grup, barcode dipisah{" "}
            <code>|</code>.
          </li>
          <li>
            Harga berupa bilangan bulat tanpa pemisah (contoh 15000). Semua
            baris harus valid, jika tidak tidak ada yang diimpor.
          </li>
        </ul>
        <div className="flex flex-wrap items-center gap-2">
          <button className={buttonClass} onClick={downloadTemplate}>
            Unduh template
          </button>
          <input
            key={fileKey}
            type="file"
            accept=".csv,text/csv"
            disabled={busy || state.kind === "report" || state.kind === "done"}
            onChange={onFile}
            aria-label="Pilih file CSV"
            className="min-h-11 text-sm"
          />
        </div>
      </div>

      {state.kind === "working" && (
        <p className="text-stone-600">{state.label}</p>
      )}

      {state.kind === "report" && (
        <div className="space-y-3 rounded-xl border border-stone-200 bg-white p-4">
          <p className="text-sm text-stone-600">{state.fileName}</p>
          <p
            role="status"
            className={`font-medium ${state.report.valid ? "text-green-800" : "text-red-800"}`}
          >
            {describeReport(state.report)}
          </p>
          {state.report.errors.length > 0 && (
            <ul className="max-h-80 divide-y divide-stone-200 overflow-auto rounded-lg border border-stone-200 text-sm">
              {state.report.errors.map((e, i) => (
                <li
                  key={i}
                  className="flex flex-col gap-0.5 p-2 sm:flex-row sm:gap-3"
                >
                  <span className="shrink-0 font-medium">
                    {locationLabel(e)}
                  </span>
                  <span className="text-stone-700">{e.message}</span>
                </li>
              ))}
            </ul>
          )}
          {hiddenErrorCount(state.report) > 0 && (
            <p className="text-sm text-stone-600">
              …dan {hiddenErrorCount(state.report)} kesalahan lainnya.
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            {state.report.valid && (
              <button
                className="min-h-11 rounded-lg bg-amber-700 px-4 text-sm font-medium text-white hover:bg-amber-800"
                onClick={() => commit(state.text)}
              >
                Impor {state.report.products} produk
              </button>
            )}
            <button className={buttonClass} onClick={reset}>
              Pilih file lain
            </button>
          </div>
        </div>
      )}

      {state.kind === "done" && (
        <div className="space-y-3 rounded-xl border border-green-200 bg-green-50 p-4">
          <p role="status" className="font-medium text-green-900">
            {describeReport(state.report)}
          </p>
          <div className="flex gap-2">
            <Link
              href="/admin/products"
              className="min-h-11 rounded-lg bg-amber-700 px-4 py-2.5 text-sm font-medium text-white hover:bg-amber-800"
            >
              Lihat produk
            </Link>
            <button className={buttonClass} onClick={reset}>
              Impor file lain
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
