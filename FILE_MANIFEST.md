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
│   │   └── 00006_product_modifier_groups.sql
│   └── internal/
│       ├── app/router.go              # rakit router + /healthz (mount auth + catalog)
│       ├── app/integration_test.go    # e2e auth + RLS (butuh TEST_DATABASE_URL)
│       ├── app/catalog_integration_test.go
│       ├── app/products_integration_test.go
│       ├── app/variants_integration_test.go
│       ├── app/modifiers_integration_test.go
│       ├── app/product_modifiers_integration_test.go
│       ├── app/price_history_integration_test.go
│       ├── app/product_csv_integration_test.go       # baru: impor/ekspor CSV
│       ├── platform/config/           # env -> Config
│       ├── platform/database/         # pgx pool, WithTenantTx
│       ├── platform/httpx/            # problem+json, JSON, middleware
│       ├── platform/ratelimit/        # limiter in-memory
│       ├── platform/authn/            # Principal di context (+ Has/Can)
│       ├── tenant/                    # README.md, domain/, infrastructure/pg/
│       ├── auth/                      # README.md, module.go, domain/ (+permission.go),
│       │                              # application/, infrastructure/ (+pg/),
│       │                              # interface/http/ (+guard.go)
│       └── catalog/                   # README.md, module.go (Handlers),
│                                      # domain/ (category, product, modifier, price_history),
│                                      # application/ (service, products, modifiers, links, price_history,
│                                      #               product_csv_format, product_csv_import),
│                                      # infrastructure/pg/ (repository, products, modifiers, modifier_links,
│                                      #                     price_history, product_csv),
│                                      # interface/http/ (handler, products, modifiers, price_history, product_csv)
└── web/
    ├── package.json, tsconfig.json, next.config.ts, postcss.config.mjs, vitest.config.mts
    ├── app/
    │   ├── layout.tsx, page.tsx, globals.css
    │   ├── login/, register/
    │   ├── dashboard/page.tsx         # redirect ke /admin
    │   └── admin/                     # layout.tsx, page.tsx, categories/page.tsx,
    │                                  # products/ (page.tsx, new/page.tsx, import/page.tsx,
    │                                  #            [id]/page.tsx, [id]/history/page.tsx),
    │                                  # modifiers/ (page.tsx, new/page.tsx, [id]/page.tsx)
    ├── components/                    # Field.tsx, SessionProvider.tsx, AdminShell.tsx,
    │                                  # ProductForm.tsx, ModifierGroupForm.tsx
    └── lib/                           # api.ts, validate.ts, category.ts, money.ts, product.ts, modifier.ts,
                                       # catalog-api.ts, price-history.ts, price-history-api.ts,
                                       # csv-import.ts, product-csv-api.ts (+ *.test.ts)
```
