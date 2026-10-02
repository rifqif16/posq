import { SessionProvider } from "@/components/SessionProvider";

export default function CashierLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <SessionProvider loginPath="/kasir/masuk">{children}</SessionProvider>;
}
