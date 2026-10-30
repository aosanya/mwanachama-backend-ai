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

`NewGateway(manager, caller, client)` is a plain `http.Handler` for an OpenAI-compatible `chat/completions` call; it is not a declared operation. The host supplies `caller`, which names the agency and the agent's agency role. The gateway resolves the AI role and its account, runs `CheckLimits`, lowers `max_tokens` to the tokens-per-call limit in force (also when the caller sent none), sets the account's model, adds the unsealed key to the outgoing request only, forwards it to the account's base URL, records the answer's `usage` once, and returns the answer unchanged. A limit answers 429 with its plain sentence and nothing leaves. An upstream error is answered 502 with a plain sentence and none of the upstream body. Streaming is refused; only `openai_compatible` accounts are served. A provider kind with no adapter is already refused when the account is saved.
