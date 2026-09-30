"use client";

import { useSession } from "@/components/SessionProvider";

export default function AdminHomePage() {
  const { profile } = useSession();
  const { user, tenant, store_ids } = profile;
  const trialEnd = tenant.trial_ends_at
    ? new Date(tenant.trial_ends_at).toLocaleDateString("id-ID", { dateStyle: "long" })
    : "-";

  return (
    <dl className="grid grid-cols-2 gap-4 rounded-xl border border-stone-200 bg-white p-4 text-sm">
      <div>
        <dt className="text-stone-500">Status</dt>
        <dd className="font-medium">{tenant.status}</dd>
      </div>
      <div>
        <dt className="text-stone-500">Trial berakhir</dt>
        <dd className="font-medium">{trialEnd}</dd>
      </div>
      <div>
        <dt className="text-stone-500">Email</dt>
        <dd className="font-medium">{user.email}</dd>
      </div>
      <div>
        <dt className="text-stone-500">Outlet</dt>
        <dd className="font-medium">{store_ids.length}</dd>
      </div>
    </dl>
  );
}
