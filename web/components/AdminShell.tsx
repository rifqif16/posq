"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession } from "./SessionProvider";

const NAV = [
  { href: "/admin", label: "Ringkasan" },
  { href: "/admin/products", label: "Produk" },
  { href: "/admin/modifiers", label: "Modifier" },
  { href: "/admin/categories", label: "Kategori" },
];

export function AdminShell({ children }: { children: React.ReactNode }) {
  const { profile, logout } = useSession();
  const pathname = usePathname();

  return (
    <div className="mx-auto max-w-4xl px-4 py-4">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-stone-200 pb-3">
        <div>
          <p className="font-semibold">{profile.tenant.name}</p>
          <p className="text-sm text-stone-600">
            {profile.user.name} · {profile.user.role}
          </p>
        </div>
        <button
          onClick={logout}
          className="min-h-11 rounded-lg border border-stone-300 bg-white px-4 text-sm hover:bg-stone-100"
        >
          Keluar
        </button>
      </header>
      <nav aria-label="Menu admin" className="flex gap-2 py-3">
        {NAV.map((item) => {
          const active =
            item.href === "/admin"
              ? pathname === item.href
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              className={`min-h-11 rounded-lg px-4 py-2.5 text-sm ${active ? "bg-amber-700 text-white" : "bg-white text-stone-700 hover:bg-stone-100"}`}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>
      <main>{children}</main>
    </div>
  );
}
