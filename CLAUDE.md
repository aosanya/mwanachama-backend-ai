# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project: mwanachama-backend-ai

Holds an agency's AI accounts, AI roles (containers of a system prompt), the mapping of roles to accounts, and the limits and usage on both. Domain logic and storage both live in this package, imported directly by whatever mounts it: no separate service. Module path `github.com/aosanya/mwanachama-backend-ai`. Not mounted anywhere yet.

## The standard

This repo follows the declared-domain standard that `mwanachama-backend-catalog` sets, and that `mwanachama-backend-insights` and `mwanachama-backend-sources` already follow.

- Objects are declared in `ai.blueprint.json`, reached from Go by `blueprint.go` (`//go:embed`, `sync.OnceValues`, `Blueprint()`, `LoadSpec(path)`, `ParseSpec(raw)`). No GORM row structs, no `AutoMigrate`.
- The HTTP surface is declared in `ai.operations.json`, reached by `operations.go`. Routes and MCP tools are built from it through `mwanachama-backend-shared`'s `dispatch`, never hand-written.
- Storage goes through `mwanachama-backend-shared/specstore`. `go.mod` requires `mwanachama-backend-shared` with `replace => ../mwanachama-backend-shared`.
- Rules (required, enums, patterns) are read off the spec by `validate.go`.
- At least two domain specs, so the module stays domain-neutral.
- `Provision(db, *spec.Spec)` and a `cmd/ddl` that prints the DDL.
- A Makefile with `build`, `test`, `test-pg`, `vet`, `clean`.
- No Go file over 300 lines.

## Invariants

- **A key is written, never read.** No operation, route, MCP tool, log line, error or test failure message returns or prints an account's key. The key lives in its own object that no operation reaches, sealed at rest with AES-256-GCM under a master key the host supplies; with no master key the module refuses to store a key. Reading a key back is a method on the manager for the gateway only, never an operation.
- **An AI role is a system prompt plus the account it uses and the agency role it speaks for.** The prompt is plain text with a size cap, edited in place, and used on the very next resolve.
- **A limit is checked before a call and counted after it.** A refusal names the limit in one plain sentence and never shows a key.
- **The module names no domain.** It knows opaque agency, member and role ids, nothing else.

## House rules

- No comments in code. A name, a type or a smaller function carries it; anything that needs explaining goes in `documentation/`.
- `documentation/` has four folders and nothing else: `1. requirements`, `2. design`, `3. implementation`, `4. qa`.
- Tests use the in-memory (SQLite) backend only. Every regression test is shown to fail without its fix.
- Commit messages carry the narration.
