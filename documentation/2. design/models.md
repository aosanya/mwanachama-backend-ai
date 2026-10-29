# Model list (HS187)

Decisions 59 and 65 of Hub Steward. The Model dropdown of an AI account is filled from data, not from code: new models are released all the time, so the list is stored and refreshed, never compiled in. One object, `model_entry`, and three manager methods behind two operations.

## What is stored

Each entry is one model name for one provider kind, in the provider's own words (the words an account's `model_name` takes). Two sources, in the enum `source`:

| Source | `agency_id` | Written by | Seen by |
|---|---|---|---|
| `provider` | the agency whose account key asked | `RefreshModels` (`ai_refresh_models`) | that agency only |
| `starting` | empty | `SetStartingModels` | every agency |

The module ships no model names. It names no provider (see the invariant in `CLAUDE.md`), so the standard starting list is data the host puts in with `SetStartingModels(ctx, provider, names)`, once at setup and again whenever it wants the standard list to move. It replaces that kind's starting list whole, and an empty list empties it. It is a Go method and not an operation: the operations are all addressed by an agency, and the standard list belongs to none. A host that wants it reachable by an administrator mounts its own route over the method.

A provider's own list is kept per agency because what a key can use can differ from key to key (fine-tuned models, restricted access). Another agency never sees it, and another agency's account refreshes as not found.

## Reading

`ai_list_models` (`GET /agencies/{agencyID}/models`, optional `provider`) answers this agency's `provider` entries and the `starting` entries, ordered by provider kind, then provider entries before starting entries, then name. A starting entry whose name the agency's refresh already lists is left out, so the provider's models stand above the starting list and nothing shows twice. An agency that has never refreshed sees the starting list alone. An unknown `provider` answers `ErrUnsupportedKind`.

## Refreshing

`ai_refresh_models` (`POST /agencies/{agencyID}/accounts/{accountID}/models/refresh`) reads the account's stored key (`ErrNoKey`, `ErrNoMasterKey` as for any use of the key), asks the provider through a `ModelLister`, and replaces this agency's `provider` entries for the account's provider kind with the answer, in one transaction. It answers the list as `ai_list_models` shows it for that kind.

The provider call sits behind `ModelLister` (`FetchModels(ctx, account, key) ModelListing`). The default, `NewHTTPModelLister`, is the only code that makes the network call; a host or a test passes another with `WithModelLister`. Per kind:

| Kind | Request | Key travels in | Names read from |
|---|---|---|---|
| `openai_compatible` | `GET {base}/models` | `Authorization: Bearer` | `data[].id` |
| `anthropic` | `GET {base}/v1/models?limit=1000`, with `anthropic-version` | `x-api-key` | `data[].id` |
| `gemini` | `GET {base}/v1beta/models?pageSize=1000` | `x-goog-api-key` | `models[].name` without `models/`, only those that can `generateContent` (or that do not say) |

The outcome maps as the account test does: 401 and 403 are `ErrModelRefused`, other non-2xx and unparseable answers are `ErrModelError`, a transport failure is `ErrModelUnreached`. Only the first page of up to 1000 names is read.

## What a refresh keeps and drops

- A failed refresh, or one that yields no usable name, leaves the list as it was. An empty answer is `ErrModelError`, not an empty list, so a provider hiccup cannot wipe the dropdown.
- Names are trimmed, de-duplicated, and dropped when blank, not valid UTF-8, over 200 characters, or holding a control character. At most 500 are kept, in the order the provider gave them.
- A name that contains the account's key is dropped, so a provider that echoes the key cannot get it into the list. The key is never in a URL or a body, and no output of either operation carries it.
- `SetStartingModels` is stricter, because its input is the host's own: a blank, over-long or control-character name, or more than 500 names, is `ErrInvalidModelList` (400) and stores nothing.

## Storage

`model_entry` is declared in `ai.blueprint.json` and added to `ai.wakala.json` and `spec/examples/research.ai.json`; a host that provisions from its own spec must add the role. The table is new, so the shared migration creates it. A host that mirrors the DDL by hand (the gateway's active migrations) must add the table there; this module does not touch that mirror. An index on `agency_id`, `provider`, `source` serves the read.

## Tests

`model_list_test.go` (order and de-duplication, agency scope, replace per kind, failures keep the list, the key, caps), `model_adapter_test.go` (each kind's request and parse against an httptest fake, statuses, key only in its header), `routes/models_test.go` and `mcp/models_test.go` (the two operations over HTTP and MCP). No test makes a real network call. Each was shown to fail with its fix removed: the key filter, the agency scope, the delete before the insert, the key moved into the URL, the de-duplication against the starting list, the provider-first order and the empty-answer refusal.
