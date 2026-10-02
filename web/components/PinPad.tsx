"use client";

import { useCallback, useEffect, useState } from "react";
import { PIN_LENGTH, appendPinDigit, removePinDigit } from "@/lib/device";

interface Props {
  disabled?: boolean;
  onSubmit: (pin: string) => Promise<void>;
}

const KEYS = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];
const keyClass =
  "min-h-16 rounded-xl border border-stone-300 bg-white text-2xl font-medium hover:bg-stone-100 disabled:opacity-50";

export function PinPad({ disabled = false, onSubmit }: Props) {
  const [pin, setPin] = useState("");
  const [busy, setBusy] = useState(false);
  const locked = disabled || busy;

  const press = useCallback(
    async (digit: string) => {
      const next = appendPinDigit(pin, digit);
      setPin(next);
      if (next.length === PIN_LENGTH) {
        setBusy(true);
        try {
          await onSubmit(next);
        } finally {
          setPin("");
          setBusy(false);
        }
      }
    },
    [pin, onSubmit],
  );

  useEffect(() => {
    if (locked) return;
    const onKey = (e: KeyboardEvent) => {
      if (/^\d$/.test(e.key)) void press(e.key);
      else if (e.key === "Backspace") setPin((p) => removePinDigit(p));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [locked, press]);

  return (
    <div className="space-y-4">
      <div
        role="status"
        aria-label={`${pin.length} dari ${PIN_LENGTH} digit`}
        className="flex justify-center gap-3"
      >
        {Array.from({ length: PIN_LENGTH }, (_, i) => (
          <span
            key={i}
            className={`size-4 rounded-full border-2 ${i < pin.length ? "border-amber-700 bg-amber-700" : "border-stone-400"}`}
          />
        ))}
      </div>
      <div className="grid grid-cols-3 gap-3">
        {KEYS.map((k) => (
          <button
            key={k}
            type="button"
            disabled={locked}
            onClick={() => void press(k)}
            className={keyClass}
          >
            {k}
          </button>
        ))}
        <span />
        <button
          type="button"
          disabled={locked}
          onClick={() => void press("0")}
          className={keyClass}
        >
          0
        </button>
        <button
          type="button"
          disabled={locked}
          onClick={() => setPin((p) => removePinDigit(p))}
          aria-label="Hapus digit"
          className={keyClass}
        >
          ⌫
        </button>
      </div>
    </div>
  );
}
