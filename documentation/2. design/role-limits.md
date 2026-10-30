# Role limits and the raise card (HS149, HS150)

The console's Limits tab asks the chat "Raise the Farmers' agent's daily tokens" and shows a form in the content area; the Usage tab offers "Raise tokens per month to 6,000,000" as a card the owner approves. The module serves the data for both. The form, the card and the approval are the console's and wakala-api's.

## Setting one AI role's limits

`ai_set_role_limits` (`PUT /agencies/{agencyID}/ai-roles/{roleID}/limits`, action `ai.role.set_limits`) is for an AI role what `ai_set_limits` is for an account: the body is the nine limits and nothing else, so a form that shows only the limits does not have to send the role's prompt, switch and name back. It replaces all nine, a limit left out is cleared (null means the limit does not apply, zero means none), and a negative number answers `ErrInvalidRole` (400). `saved_by` comes from the mount's caller, never the request. A change is a revision exactly like a change through `ai_update_role`, and saving the same limits again keeps no new one. The next resolve and the next limit check read it.

A form that raises one limit reads the role (`ai_get_role`) first and sends the other eight back.

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
