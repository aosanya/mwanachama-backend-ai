# Role limits and the raise card (HS149, HS150)

The console's Limits tab asks the chat "Raise the Farmers' agent's daily tokens" and shows a form in the content area; the Usage tab offers "Raise tokens per month to 6,000,000" as a card the owner approves. The module serves the data for both. The form, the card and the approval are the console's and wakala-api's.

## Setting one AI role's limits

`ai_set_role_limits` (`PUT /agencies/{agencyID}/ai-roles/{roleID}/limits`, action `ai.role.set_limits`) is for an AI role what `ai_set_limits` is for an account: the body is the nine limits and nothing else, so a form that shows only the limits does not have to send the role's prompt, switch and name back. It replaces all nine, a limit left out is cleared (null means the limit does not apply, zero means none), and a negative number answers `ErrInvalidRole` (400). `saved_by` comes from the mount's caller, never the request. A change is a revision exactly like a change through `ai_update_role`, and saving the same limits again keeps no new one. The next resolve and the next limit check read it.

A form that raises one limit reads the role (`ai_get_role`) first and sends the other eight back.

## What an absent key does

Pinned by `routes/limit_writes_route_test.go`, which drives every row below through the route table (and `ai_set_limits`, `ai_set_role_limits` again as MCP tools). A limit is a pointer: a key that is absent and a key sent as `null` decode the same way, so no write can say "keep this one". Every write that carries limits replaces all nine; send the held ones back (read the account or role first).

| Tool | A limit key absent | A limit key `null` | A limit key `0` | Other keys left out |
| --- | --- | --- | --- | --- |
| `ai_set_limits` | cleared to null | cleared to null | stored as 0 | not accepted: the body is the nine limits and nothing else |
| `ai_set_role_limits` | cleared to null | cleared to null | stored as 0 | not accepted, as above; the change is a revision |
| `ai_update_account` | cleared to null | cleared to null | stored as 0 | `name`, `provider`, `base_url`, `model_name` and `status` are required and a body without them is refused (400) with the limits untouched; `chat_enabled` and `chat_label` are kept |
| `ai_update_role` | cleared to null | cleared to null | stored as 0 | `description` and `fine_tuning` are blanked; `name` and `status` are required; `use_agency_instructions` is kept; the change is a revision when the fine-tuning, the switch or a limit differs |
| `ai_create_account` | starts as null | starts as null | stored as 0 | `chat_enabled` starts off and `chat_label` empty |
| `ai_create_role` | starts as null | starts as null | stored as 0 | `use_agency_instructions` defaults to on; `description` and `fine_tuning` start empty |

A stored 0 and a stored null differ only in what reads back: `ai_check_limits` skips a limit whose maximum is 0 (`usage.go`, `if max == 0 { continue }`), so both are unenforced. The trap is the first column: a form that sends only the limit it changed silently clears the other eight, `tokens_per_test` and `scenarios_at_once` included, and on a role that clearing is also saved as a revision. The holder of a form sends all nine, with the held values for the ones the person did not touch.

The description of `ai_set_limits`, `ai_update_account`, `ai_update_role` and `ai_set_role_limits` says it in one sentence a caller reads before it calls: "A limit left out of the body is cleared, so send the ones to keep." `mcp/limit_writes_description_test.go` pins that the four carry it.

## What a refusal carries

`ai_check_limits` answers a refused call with its sentence in `reason` as before, and now also:

| Field | Holds |
| --- | --- |
| `limit` | The key the limit is saved under: `tokens_per_day`, `tokens_per_week`, `tokens_per_month`, `requests_per_minute`, `requests_per_day` or `requests_per_week`. |
| `owner` | `role` or `account`: whose limit stopped the call. A role's limit is looked at first. |
| `max` | The limit's maximum. |
| `used` | The count against it. |

The card therefore reads which limit to offer to raise, on whom (`ai_set_role_limits` for `role`, `ai_set_limits` for `account`), from what, and how far it has gone. An allowed call carries none of the four. A refusal for a test run's token budget (`budget_reached`) carries none either, since that limit belongs to the run.

## The same key on the other reads

Each row of `ai_list_usage` (`counts`) and of `ai_resolve_role` (`limits`) carries `field`, the same key, beside the human `limit` name (`daily token`, `monthly token`), so a row's Raise button knows what to send. `tokens per call` has `field` `tokens_per_call` on the resolution, where it has a row.
