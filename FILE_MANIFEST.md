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
│   ├── go.mod / go.sum
│   ├── cmd/api/main.go                # entrypoint HTTP
│   ├── cmd/migrate/main.go            # goose up|down|status
│   ├── db/embed.go
│   ├── db/migrations/
│   │   ├── 00001_identity.sql
│   │   └── 00002_categories.sql       # baru (fase 2)
│   └── internal/
│       ├── app/router.go              # rakit router + /healthz (mount auth + catalog)
│       ├── app/integration_test.go    # e2e auth + RLS (butuh TEST_DATABASE_URL)
│       ├── app/catalog_integration_test.go   # baru: kategori, otorisasi, isolasi
│       ├── platform/config/           # env -> Config
│       ├── platform/database/         # pgx pool, WithTenantTx
│       ├── platform/httpx/            # problem+json, JSON, middleware
│       ├── platform/ratelimit/        # limiter in-memory
│       ├── platform/authn/            # baru: Principal di context
│       ├── tenant/                    # README.md, domain/, infrastructure/pg/
│       ├── auth/                      # README.md, module.go, domain/ (+permission.go),
│       │                              # application/, infrastructure/ (+pg/),
│       │                              # interface/http/ (+guard.go)
│       └── catalog/                   # baru: README.md, module.go, domain/, application/,
│                                      # infrastructure/pg/, interface/http/
└── web/
    ├── package.json, tsconfig.json, next.config.ts, postcss.config.mjs, vitest.config.mts
    ├── app/
    │   ├── layout.tsx, page.tsx, globals.css
    │   ├── login/, register/
    │   ├── dashboard/page.tsx         # redirect ke /admin
    │   └── admin/                     # baru: layout.tsx, page.tsx, categories/page.tsx
    ├── components/                    # Field.tsx, SessionProvider.tsx, AdminShell.tsx
    └── lib/                           # api.ts, validate.ts, category.ts, catalog-api.ts (+ *.test.ts)
```
