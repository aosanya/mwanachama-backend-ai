# AI roles (HS125)

An AI role is a named container for a system prompt, per agency. `ai_role` holds name (unique per agency), description, `fine_tuning` (at most 20 000 characters), `use_agency_instructions` (on unless switched off), status and the same five limits as an account (zero means none). `ai_role_revision` holds the values a role had before each save.

## The prompt

`Instructions(agency, role)` reads the agency's instructions live through the host's `InstructionSource` on every call, never copying them, and follows them with the fine-tuning. With the switch off, or with no source supplied, the fine-tuning alone is returned, and the fine-tuning is then required.

## Limits

`CheckLimits` loads the stored role when `role_id` names one (an unknown id falls back to the `role_limits` in the request). A role's usage is checked against the role's limits and the account's total against the account's, so for each of the five limits the lower non-zero applies; tokens per call returns the lower of the two.

## History

A save that changes the fine-tuning, the switch or a limit adds one revision holding the previous values, the editor (`saved_by`, bound from the caller) and the time. A restore copies a revision back and is itself a saved edit, so it adds a revision. Edits to name, description or status add none.

Not here: the account and agency-role mapping, and role resolution (HS126).
