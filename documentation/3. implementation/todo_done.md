# mwanachama-backend-ai: done

| Task | Done | Verified |
|---|---|---|
| Repository created, public, default branch master, with the requirements and house rules | 2026-10-27 | `gh repo view` |
| HS122 AI accounts: add, change, list, remove, and a key written and never read (sealed AES-256-GCM, Go-only read) | 2026-10-27 | `go vet ./... && go test ./...`; leak, unsealed-storage, agency-scope and master-key mutations each fail the matching test |
| HS123 Test an AI account: one short request with the key through an injected prober, last result and time stored, one of five fixed sentences | 2026-10-27 | `go vet ./... && go test ./...`; a result carrying the key fails TestAccountTestEachReason |
