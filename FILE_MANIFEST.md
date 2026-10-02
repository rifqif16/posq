# FILE_MANIFEST

```text
.
├── README.md
├── FILE_MANIFEST.md
├── docker-compose.yml                 # PostgreSQL lokal + init role
├── .env.example
├── .gitignore
├── deploy/postgres/init.sql           # role posq_app (tanpa BYPASSRLS)
├── docs/adr/0001-auth-session-and-tenant-lookup.md
├── api/
│   ├── go.mod / go.sum                # module github.com/rifqif16/posq/api
│   ├── cmd/api/main.go                # entrypoint HTTP
│   ├── cmd/migrate/main.go            # goose up|down|status
│   ├── db/embed.go
│   ├── db/migrations/
│   │   ├── 00001_identity.sql
│   │   ├── 00002_categories.sql
│   │   ├── 00003_products.sql
│   │   ├── 00004_variants.sql
│   │   ├── 00005_modifiers.sql
│   │   ├── 00006_product_modifier_groups.sql
│   │   ├── 00007_inventory.sql
│   │   ├── 00008_staff.sql
│   │   └── 00009_devices.sql                         # baru: devices, kolom kunci PIN, auth_find_device
│   └── internal/
│       ├── app/router.go              # rakit router + /healthz (auth, staf, perangkat, catalog, inventory)
│       ├── app/*_integration_test.go  # integration (butuh TEST_DATABASE_URL); baru: device_integration_test.go
│       ├── platform/                  # config, database, httpx, ratelimit, authn, pagination
│       ├── tenant/                    # README.md, domain/, infrastructure/pg/
│       ├── auth/                      # README.md, module.go, staff_module.go, device_module.go (baru),
│       │                              # domain/, application/ (+device.go, pin_login.go),
│       │                              # infrastructure/ (+pg/device.go), interface/http/ (+device.go)
│       ├── catalog/                   # README.md, module.go, domain/, application/, infrastructure/pg/, interface/http/
│       └── inventory/                 # README.md, module.go, domain/, application/, infrastructure/pg/, interface/http/
└── web/
    ├── package.json, tsconfig.json, next.config.ts, postcss.config.mjs, vitest.config.mts
    ├── app/
    │   ├── layout.tsx, page.tsx, globals.css
    │   ├── login/, register/
    │   ├── dashboard/page.tsx         # redirect ke /admin
    │   ├── kasir/                     # baru: masuk/page.tsx (login PIN), (app)/layout.tsx + page.tsx (placeholder)
    │   └── admin/                     # layout.tsx, page.tsx, categories, products/, inventory/, modifiers/, staff/,
    │                                  # devices/page.tsx                     # baru
    ├── components/                    # Field, SessionProvider, AdminShell, ProductForm, ModifierGroupForm,
    │                                  # StockMovementForm, StaffForm, StaffSecretForm, PinPad (baru)
    └── lib/                           # api.ts, validate.ts, category.ts, money.ts, product.ts, modifier.ts,
                                       # catalog-api.ts, price-history*.ts, csv-import.ts, product-csv-api.ts,
                                       # stock.ts, inventory-api.ts, staff.ts, staff-api.ts,
                                       # device.ts, device-api.ts (baru) (+ *.test.ts)
```
