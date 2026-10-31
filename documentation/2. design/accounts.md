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

A database provisioned before these fields has no such columns, and the shared migration only creates missing tables (the open S52 bug in `mwanachama-backend-shared`), so such a database needs `alter table <accounts table> add column chat_enabled boolean default false` and `add column chat_label text default ''` run once. The same holds for the three test fields of the testing section: `last_test_prompt text default ''`, `last_test_reply text default ''` and `last_test_ms bigint default 0`. A row that holds NULL in any of these columns reads as false, empty or zero.

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

### Testing a key that is not saved yet

`account.test` takes an optional body `{"key": "..."}`, the same shape as `account.key_set`. With no key, or an empty one, the saved key is used and the result is stored on the account as below. With a key, the one request is made with that key instead and the result is returned in the answer's `last_test_*` fields but stored nowhere: the saved key, the stored result and the account's `updated_at` stay as they were, because a stored result belongs to the key it was measured with (see the key section) and the typed key may never be saved. The typed key is never sealed, returned, logged or put in an error, and a reply that contains it is dropped like one that contains the saved key. A typed key that is blank or longer than 4096 bytes is refused with `invalid key` before any request is made. No master key is needed, since nothing is stored.

An HTTP caller of `POST /agencies/{agencyID}/accounts/{accountID}/test` now sends a JSON body, `{}` when it has no key to try; an empty body is no longer accepted, as for every other operation that takes a body. The MCP tool `ai_test_account` takes `key` as an optional argument beside `accountID`.

The result stored on the account (`last_test_result`, `last_tested_at`) is exactly one of `working`, "no key is set for this account", "the model refused the key" (401 or 403), "the model could not be reached" (transport failure) or "the model answered with an error" (any other status).

### What was pinged and what answered

So the owner can see the test and not only its verdict, the account also keeps three read-only fields, written only by `account.test`:

| JSON field | Type | Holds |
| --- | --- | --- |
| `last_test_prompt` | string | The message sent: "Reply with the single word OK." Empty when nothing was sent, which is the no-key outcome. |
| `last_test_reply` | string | What the model answered. Empty unless the outcome is `working`. |
| `last_test_ms` | integer | How long the call took, in whole milliseconds, rounded down, never negative. A refused, unreached or errored call still records its time (an unreached call that waited out the 20 second client timeout reads 20000). Zero when never tested and for the no-key outcome, where no call was made. |

All three are always present in the account's JSON (empty string, empty string, 0 for an account never tested), on create, get, list, update and test. An `update_account` or `create_account` body that carries them is accepted and ignored: a create clears them and an update leaves the stored ones alone, so a console that sends back the whole record it read cannot forge or lose a test.

`Prober.Probe` returns a `ProbeResult`: the outcome, the model's reply text and the elapsed time. The HTTP prober takes the reply from `Call` (the adapter's parsed `Reply.Text`) and times the call itself; a host's own prober fills the same three. The account manager, not the prober, decides what is stored, so every prober gets the same treatment:

- The reply is kept only for the `working` outcome.
- Control characters (newlines and tabs included, so two lines run together) and invalid UTF-8 are removed, the text is trimmed, cut to 120 characters (counted as characters, not bytes) and trimmed again.
- A reply that contains the account's key anywhere is dropped whole, before any cutting, so a provider that echoes what it received can never put the key, or the front of it, into a returned field. Replying with the key is a provider's choice; showing it is not ours.

Replacing the key clears all five test fields (`last_test_result`, `last_tested_at` and the three above) in the stored row and in the answer, for the reason in the key section: a result belongs to the key it was measured with. A replacement that is refused changes nothing. `Account.ForgetTest` is the one place that clears them, used by create and by the key replacement.

The key is still never returned, logged or put in an error. The old guarantee that the stored result could not carry anything the model sent back no longer holds for `last_test_reply`; the reply is what the model said, so the limits above (kept only on `working`, cleaned, 120 characters, dropped if it holds the key) are what keep it harmless.

## Limits and usage (HS124)

The `usage` object holds one row per account, optional AI role id and window kind (minute, day or month, all UTC), counting requests, tokens in and tokens out. `usage.record` bumps the three windows of the current instant in one call. `limits.check` takes the account, an optional AI role id and the role's limits as a parameter (the AI role object is HS125) and answers allowed, or refused with one sentence naming whose limit and which, for example "this AI role's daily token limit is reached". It looks at the role's limits first, then the account's, so the lower of the two applies per limit; a limit of zero is none. A call is refused once the count has reached the limit, so the call that crosses it is let through and the next is the first refused. Role limits count only that role's rows, account limits count every row of the account. Tokens per call is a clamp: the answer carries the lower non-zero of the two as `max_tokens`. `usage.list` returns each limit beside its count, for the account or for one role (`ListRoleUsage`). Counters are rows, so a new manager over the same database sees them; every row carries the agency, so agencies count apart.

The window kind is stored in the column `window_kind`, and the row's storage key is still `agency|account|role|kind|start`. The column was first called `window`, which Postgres reserves: the shared migration writes column names unquoted, so on Postgres the table was never created and mounting for a hub failed with `syntax error at or near "window"`, while SQLite accepted it and every test passed. `usage.record` still answers with the row under the JSON key `window`, so nothing that reads the answer changes; only the Go field (`Usage.WindowKind`) and the column carry the new name. `limits.check` and `usage.list` answer no row, so they do not show it.

No declared table, column or index column may be a Postgres reserved word. `postgres_reserved_test.go` builds each shipped spec's DDL for the postgres dialect and fails on any unquoted table, column, primary key column, index or index column that is on the reserved list, which holds the words Postgres rejects as a column name: the reserved class plus the reserved-as-function-or-type class (`left`, `right`, `full`, `like`, `is`, `join`, `cross`, `natural`, `outer` and the rest). It also fails when it cannot read a statement, and checks that it reads every declared table and column, so a change to the generated DDL cannot make it pass by seeing nothing.

## Chatting with an account (`account.chat`)

`account.chat` lets a console run a tool-using chat against an account's model. It is stateless: the caller keeps the conversation, sends all of it each turn, and gets back one assistant message. When that message carries `tool_calls`, the caller runs them and sends the whole conversation again with the results as `tool` messages. The REST address is `POST /agencies/{agencyID}/accounts/{accountID}/chat` and the MCP tool is `ai_chat_account`; its arguments are the agency, `accountID`, `messages`, and optionally `tools` and `max_tokens`, and nothing else is accepted.

A chat answer is returned before its usage is written. The usage is recorded in the background, one write at a time per account, because each write takes several round trips to the database. Any later read of that account's limits or usage, including the next chat call, waits for the pending write first, so a limit still stops the call after the one that reached it. A restart or crash can lose the usage of an answer whose write had not finished.

| Body field | Holds |
| --- | --- |
| `messages` | The conversation so far, oldest first, at most 200 messages and 1 MiB of content. Each has `role` (`system`, `user`, `assistant` or `tool`) and `content`. An assistant message that asked for tools carries `tool_calls` (`id`, `name`, `arguments` as an object); a `tool` message answers one with its `tool_call_id`. A user or system message needs content; an assistant message needs content or tool calls. |
| `tools` | Optional, at most 128. Each has `name` (unique), `description` and `parameters`, a JSON schema object; none given means an empty object schema. |
| `max_tokens` | Optional. Default 1024, never more than 16384, and never more than the account's tokens per call. |

The answer is `{"message": {"role": "assistant", "content", "tool_calls"}, "input_tokens", "output_tokens", "finish_reason"}`. `finish_reason` is the provider's own word for why it stopped (`stop`, `length`, `end_turn`, `max_tokens`) and is left out when the provider gave none; a model that reasons before it answers can spend all of `max_tokens` on the reasoning and send back an empty `content` with `length` here. `content` is empty when the model only asked for tools. A tool call the provider gave no id gets `call-1`, `call-2` and so on, so a result can always name it, and `arguments` is always an object (`{}` when the provider's were empty or not JSON).

In order, the operation: finds the account in the caller's agency only (another agency's account id answers `account not found`, as for every account operation); refuses an account that is turned off or a provider that cannot chat; checks the conversation; reads the saved key (never returned); checks the account's limits as `limits.check` does and refuses at a limit; sends the conversation through the provider's adapter with an HTTP client the host may set with `WithChatClient` (default timeout two minutes); and, only when the model answered, counts one request and its tokens in the account's usage. A failed call counts nothing.

Providers: `openai_compatible` and `anthropic` chat. `gemini` answers "this provider cannot chat with tools yet", and the request is never sent: Gemini matches a tool result to its call by name, and the contract here carries only ids. The adapter refuses a conversation it cannot carry rather than dropping the history.

| Refusal | Status | Sentence |
| --- | --- | --- |
| Account missing, or another agency's | 404 | account not found |
| No key saved | 404 | this account has no key |
| No master key configured | 409 | no master key is configured, so no key can be stored |
| Account turned off | 409 | this account is turned off |
| Provider cannot chat | 400 | this provider cannot chat with tools yet |
| Conversation malformed | 400 | invalid chat request: followed by what is wrong |
| A limit is reached | 429 | this account is at its limit: followed by the limit's sentence |
| Model refused the key | 502 | the model refused the key |
| Model not reached | 502 | the model could not be reached |
| Model answered with an error | 502 | the model answered with an error |

Nothing of the conversation is stored: no message, tool call, result or reply is written anywhere, and the only trace of a chat is the usage rows. The key is never returned, logged or put in an error, including when the provider fails: an error is one of the fixed sentences above, and what the provider said is discarded.

`chat_test.go`, `chat_refusals_test.go` and `chat_secrets_test.go` pin this against a fake provider, including a scan of the result, the error text and captured log output for the key and a scan of every stored row for the conversation's text; `routes/chat_route_test.go` and `mcp/chat_tool_test.go` pin the statuses and the tool's arguments.
