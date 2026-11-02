# AI roles (HS125)

An AI role is a named container for a system prompt, per agency. `ai_role` holds name (unique per agency), description, `fine_tuning` (at most 20 000 characters), `use_agency_instructions` (on unless switched off), status and the same five limits as an account (zero means none). `ai_role_revision` holds the values a role had before each save.

## The prompt

`Instructions(agency, role)` reads the agency's instructions live through the host's `InstructionSource` on every call, never copying them, and follows them with the fine-tuning. With the switch off, or with no source supplied, the fine-tuning alone is returned, and the fine-tuning is then required.

## Limits

`CheckLimits` loads the stored role when `role_id` names one (an unknown id falls back to the `role_limits` in the request). A role's usage is checked against the role's limits and the account's total against the account's, so for each of the five limits the lower non-zero applies; tokens per call returns the lower of the two.

## History

A save that changes the fine-tuning, the switch or a limit adds one revision holding the previous values, the editor (`saved_by`, bound from the caller) and the time. A restore copies a revision back and is itself a saved edit, so it adds a revision. Edits to name, description or status add none.

Not here: the account and agency-role mapping, and role resolution (HS126).

## Mapping and resolving (HS126)

An AI role carries `account_id` and `agency_role_id`, both opaque. `ai_map_role` sets them together; the account must belong to the same agency, otherwise the account is answered as not found. An empty id clears the mapping.

`ai_resolve_role` (by AI role id) and `ai_resolve_agency_role` (by agency role id) return the prompt from `Instructions`, the account without its key, the prompt's `parts` (a list of `{source, text}`; sources `seat_description`, `objectives`, `values_rules`, `work_item_instructions` when the instruction source also implements `PartsSource`, otherwise one `agency` part from the whole agency text, then `fine_tuning` for the role's own text; empty parts are left out) and five limits in force: the lower non-zero of the role's and the account's, each with a `source` of `role`, `account` (a tie reads `role`) or `none` when both are zero, with the role's own count so far for each window. Tokens per call has no count. A role with no account of its own uses the agency's default account (`account_source` `default`; a role's own account reads `role`), and the limits in force are read off that account. With no default marked either, it returns the line "no account is mapped" and no account or limits. See `hub-models.md`. The key is never part of a resolution; only `KeyReader` reaches it.

## The model gateway (HS127)

`NewGateway(manager, caller, client)` is a plain `http.Handler` for an OpenAI-compatible `chat/completions` call; it is not a declared operation. The host supplies `caller`, which names the agency and the agent's agency role. The gateway resolves the AI role and its account, takes a hold with `HoldRequest` (see below), lowers `max_tokens` to the tokens-per-call limit in force (also when the caller sent none), sets the account's model, adds the unsealed key to the outgoing request only, forwards it to the account's base URL, settles the hold with the answer's `usage` once, and returns the answer unchanged. A limit answers 429 with its plain sentence and nothing leaves. An upstream error is answered 502 with a plain sentence and none of the upstream body. Streaming is refused; only `openai_compatible` accounts are served. A provider kind with no adapter is already refused when the account is saved.

### Calls made at the same moment (HS623)

`CheckLimits` reads the count and `RecordUsage` writes it after the call, so calls made together all read the same count and all passed. The gateway therefore asks `HoldRequest` instead. It checks the limits and, for a call that passes, takes one hold for that agency, account and AI role in the same step, so a second call checking at that moment sees the first. A request limit (per minute, per day, per week) is compared with the recorded requests plus the holds in its scope: an account limit counts the holds of every AI role on the account, an AI role limit counts that role's holds. Token limits are unchanged; the call that crosses a token limit is still let through.

The hold ends when the call does. `Settle` records the usage and drops the hold in one step, so a finished call counts once and never twice; `Release` drops it without recording, and the gateway defers it, so a call that fails (no key, upstream down, upstream error, a panic) leaves nothing behind and is not counted. `Settle` and `Release` after either has run do nothing.

Holds live in the memory of the process that made them. A host that runs several gateway processes over one database still has the old gap between those processes; one process, the case today, has none. Checks and settles on one agency and account are serialised by a small fixed set of locks, so unrelated accounts rarely wait on each other. `CheckLimits` alone is unchanged and ignores holds; the console chat still uses it.
