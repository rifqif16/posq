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
│   │   └── 00007_inventory.sql                       # baru: stock_movements, stock_levels
│   └── internal/
│       ├── app/router.go              # rakit router + /healthz (mount auth, catalog, inventory)
│       ├── app/integration_test.go    # e2e auth + RLS (butuh TEST_DATABASE_URL)
│       ├── app/catalog_integration_test.go
│       ├── app/products_integration_test.go
│       ├── app/variants_integration_test.go
│       ├── app/modifiers_integration_test.go
│       ├── app/product_modifiers_integration_test.go
│       ├── app/price_history_integration_test.go
│       ├── app/product_csv_integration_test.go
│       ├── app/inventory_integration_test.go         # baru
│       ├── platform/config/           # env -> Config
│       ├── platform/database/         # pgx pool, WithTenantTx
│       ├── platform/httpx/            # problem+json, JSON, middleware
│       ├── platform/ratelimit/        # limiter in-memory
│       ├── platform/authn/            # Principal di context (+ Has/Can)
│       ├── platform/pagination/       # baru: cursor kunci/waktu, Clamp (+ test)
│       ├── tenant/                    # README.md, domain/, infrastructure/pg/
│       ├── auth/                      # README.md, module.go, domain/ (+permission.go),
│       │                              # application/, infrastructure/ (+pg/),
│       │                              # interface/http/ (+guard.go)
│       ├── catalog/                   # README.md, module.go (Handlers),
│       │                              # domain/, application/, infrastructure/pg/, interface/http/
│       └── inventory/                 # baru: README.md, module.go,
│                                      # domain/ (qty, movement), application/ (service),
│                                      # infrastructure/pg/ (repository), interface/http/ (handler)
└── web/
    ├── package.json, tsconfig.json, next.config.ts, postcss.config.mjs, vitest.config.mts
    ├── app/
    │   ├── layout.tsx, page.tsx, globals.css
    │   ├── login/, register/
    │   ├── dashboard/page.tsx         # redirect ke /admin
    │   └── admin/                     # layout.tsx, page.tsx, categories/page.tsx,
    │                                  # products/ (page, new, import, [id], [id]/history),
    │                                  # inventory/ (page.tsx, movements/page.tsx),
    │                                  # modifiers/ (page, new, [id])
    ├── components/                    # Field.tsx, SessionProvider.tsx, AdminShell.tsx, ProductForm.tsx,
    │                                  # ModifierGroupForm.tsx, StockMovementForm.tsx
    └── lib/                           # api.ts, validate.ts, category.ts, money.ts, product.ts, modifier.ts,
                                       # catalog-api.ts, price-history.ts, price-history-api.ts,
                                       # csv-import.ts, product-csv-api.ts, stock.ts, inventory-api.ts (+ *.test.ts)
```
