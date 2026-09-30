import { AdminShell } from "@/components/AdminShell";
import { SessionProvider } from "@/components/SessionProvider";

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <AdminShell>{children}</AdminShell>
    </SessionProvider>
  );
}
