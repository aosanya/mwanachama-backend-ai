# Accounts

HS122. An account is an agency's standing with one model provider. Two objects: `account` (everything an operation may show, including `key_set`) and `account_key` (the sealed key, in a table of its own that no operation reaches).

## Agency scope

The agency id is an opaque string the host supplies in the address (`/agencies/{agencyID}/accounts`). It is bound by the address, never read from a body, and every read and write filters on it. Another agency's account answers as not found. A name is unique within one agency (a unique index on agency and name, plus a check that answers `ErrDuplicateName`).

## The key

`PUT .../key` replaces the key and answers the account with `key_set` true. Nothing returns it. The key is sealed with AES-256-GCM under a 32-byte master key passed with `WithMasterKey`; the stored value is a random nonce followed by the ciphertext, base64. With no master key `SetAccountKey` answers `ErrNoMasterKey` and stores nothing. The gateway reads a key through `KeyReader.AccountKey(ctx, agencyID, accountID)`, a Go method that no operation declares.

Replacing a key clears `last_test_result` and `last_tested_at` on the account, in the stored row and in the answer, because a result belongs to the key it was measured with: the page shows an untested account until the new key is tested. A replacement that is refused (empty, too long, no master key) changes nothing, the old result included.

Tests pin that the key is absent from every operation's output, every error and the log, on both the manager and the HTTP route table, and that the stored value is not the plain key.

## Provider agnostic

`provider`, `base_url` and `model_name` are all required, with no default. A provider kind with no adapter (`HasAdapter`) is refused with `ErrUnsupportedKind` on create and on update. The first kind is `openai_compatible`.

## Chat use (decision 60 of Hub Steward)

The hub console says which of a hub's accounts the studio chat may use, and what the chat's model picker calls each, so the same GLM-5.3 reached through Hugging Face and through Baseten can read "GLM-5.3 (Hugging Face)" and "GLM-5.3 (Baseten)". Two fields on `account` carry it:

- `chat_enabled`: true or false, false for a new account and for a row stored before the field existed. It says whether the studio's chat may use the account. It is separate from `status`: `status` says whether the account may be used at all, `chat_enabled` says whether the chat's picker lists it.
- `chat_label`: the name the picker shows, empty for none (the picker then shows `model_name`). At most 60 characters, counted as characters and not bytes, valid UTF-8, no control character (a newline, a tab, escape and the other C0 and C1 controls are all refused). Spaces and punctuation are fine. The rule is the pattern `chat_label` in `patterns.go`, named by the blueprint's `matches`, so it runs in `valid()` on create and on update and a refusal answers `ErrInvalidAccount` with "chat_label does not match chat_label". A refused write changes nothing.

Both fields are sent and returned like any other field of the account: create, update, get and list. The console's `update_account` call sends the whole record, and an update replaces name, provider, address, model, status and limits. The two chat fields are different in one way: an update whose body leaves a key out keeps the stored value, one key at a time. A body naming `chat_enabled` and not `chat_label` changes the switch and keeps the label; `"chat_label": ""` clears the label. This is so an older client, or a tool call that predates the fields, cannot switch the chat off by not knowing them. A Go caller handing `UpdateAccount` an `Account` value always replaces both, since a plain bool cannot say it was left out; `Account.UnmarshalJSON` (`models/account_json.go`) records which keys a body carried, and `Account.WithChatOf` applies that. The same decoder still refuses a key the account does not declare, as the dispatch layer did before. `SetAccountLimits` and `SetAccountKey` never touch the chat fields.

The module does not enforce that at least one account is chat-enabled. A hub with none is a valid state here; the console owns that rule and refuses to save a hub with no chat account. Keeping it out of the module means a host that mounts the module for something other than a chat is not bound to it, and removing or disabling the last chat account is never refused by the module.

A database provisioned before these fields has no such columns, and the shared migration only creates missing tables (the open S52 bug in `mwanachama-backend-shared`), so such a database needs `alter table <accounts table> add column chat_enabled boolean default false` and `add column chat_label text default ''` run once. A row that holds NULL in either column reads as false and empty.

## Address policy

The module names no provider and no host, so it cannot decide which addresses are safe. A host that mounts it where the address is called from its own network passes `WithBaseURLPolicy(func(baseURL string) error)`. `valid()` calls it on every write of an account: create, update and limits. A refusal answers `ErrInvalidAccount` followed by the policy's own sentence and stores nothing. With no policy any address is accepted, as before.

The policy runs at write time only. An account stored before a policy tightened keeps its address until its next write, and that write is refused until the address is changed, including a limits change. A host that tightens a policy and needs the old addresses dead at once checks the stored address itself before the call.

The policy receives the address and nothing else, so the key can never reach its error.

## Not built here

The test operation, usage counting and limit enforcement are HS123 and HS124. The five limits and the last-test fields are stored only; zero means none.

## Second domain

`spec/examples/research.ai.json` names the same two roles `connection` and `credential`, so the module is exercised under words that differ from the default domain.

## Testing an account (HS123)

`account.test` sends one request, "Reply with the single word OK.", to the account's own address with its stored key, through a `Prober` the host injects with `WithProber`. The module ships an HTTP prober for `openai_compatible` (`chat/completions`); tests use a fake whose mode picks the outcome. With none injected the HTTP prober is used.

The result stored on the account (`last_test_result`, `last_tested_at`) is exactly one of `working`, "no key is set for this account", "the model refused the key" (401 or 403), "the model could not be reached" (transport failure) or "the model answered with an error" (any other status). The prober returns an outcome code, never text, so nothing the model sent back and no part of the key can reach the stored result or the answer.

## Limits and usage (HS124)

The `usage` object holds one row per account, optional AI role id and window kind (minute, day or month, all UTC), counting requests, tokens in and tokens out. `usage.record` bumps the three windows of the current instant in one call. `limits.check` takes the account, an optional AI role id and the role's limits as a parameter (the AI role object is HS125) and answers allowed, or refused with one sentence naming whose limit and which, for example "this AI role's daily token limit is reached". It looks at the role's limits first, then the account's, so the lower of the two applies per limit; a limit of zero is none. A call is refused once the count has reached the limit, so the call that crosses it is let through and the next is the first refused. Role limits count only that role's rows, account limits count every row of the account. Tokens per call is a clamp: the answer carries the lower non-zero of the two as `max_tokens`. `usage.list` returns each limit beside its count, for the account or for one role (`ListRoleUsage`). Counters are rows, so a new manager over the same database sees them; every row carries the agency, so agencies count apart.

The window kind is stored in the column `window_kind`, and the row's storage key is still `agency|account|role|kind|start`. The column was first called `window`, which Postgres reserves: the shared migration writes column names unquoted, so on Postgres the table was never created and mounting for a hub failed with `syntax error at or near "window"`, while SQLite accepted it and every test passed. `usage.record` still answers with the row under the JSON key `window`, so nothing that reads the answer changes; only the Go field (`Usage.WindowKind`) and the column carry the new name. `limits.check` and `usage.list` answer no row, so they do not show it.

No declared table, column or index column may be a Postgres reserved word. `postgres_reserved_test.go` builds each shipped spec's DDL for the postgres dialect and fails on any unquoted table, column, primary key column, index or index column that is on the reserved list, which holds the words Postgres rejects as a column name: the reserved class plus the reserved-as-function-or-type class (`left`, `right`, `full`, `like`, `is`, `join`, `cross`, `natural`, `outer` and the rest). It also fails when it cannot read a statement, and checks that it reads every declared table and column, so a change to the generated DDL cannot make it pass by seeing nothing.
