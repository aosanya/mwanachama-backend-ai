# A thing's stable identity

`Manager` embeds the shared `identity.Owner` (shared S56). The AI module never re-mints an id, so the raw is built from the row id itself and survives for as long as the row does.

`StableIdentity(ctx, object, id)` answers `ai/<instance>/<object>-<id>` for the three objects an operator names: `account`, `ai_role` and `ai_role_revision`. The instance is the spec's instance, the same one that prefixes the tables. The text before the first dash names the object, so a raw says which object it is.

`Resolve(ctx, raw)` answers the `(object, id)`:

- `identity.ErrInvalidRaw` for a string that does not parse;
- `identity.ErrNotOwned` for another module's raw or another instance's;
- `identity.ErrNotFound` for a prefix the module does not mint (including `account_key`, so the sealed key table cannot be probed), an id with no row, or an id that belongs to another object;
- `ErrNoStableIdentity` from `StableIdentity` for an object that carries no identity (`account_key`, `usage`, `test_run_usage`, `model_entry`).

## Scope

A manager is built over one spec, and a mount gives each agency its own table set (wakala-api builds the spec from the instance's table slug), so `Resolve` reaches only that agency's rows. The owner interface carries no agency id. A host that keeps several agencies in one table set gets the row of any of them from a raw, and must scope the raw itself.

## Arguments that carry a raw

`ai.operations.json` marks `accountID` as `ai.account`, `roleID` as `ai.ai_role`, `revisionID` as `ai.ai_role_revision` and `agencyRoleID` as `agency.role` (shared S57). `agencyID` is the scope and `testRunID` belongs to the run store, so neither is marked. The arguments inside a request body are not marked: a `whole` body binds no single value.

Tests: `identity_owner_test.go`, `operations_refers_test.go`.
