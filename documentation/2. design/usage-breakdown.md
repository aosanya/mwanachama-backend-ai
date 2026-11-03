# Usage by day, kind of work and person (HS741, HS742)

Decision 162 of the hub project: the studio's Usage pages show where an agency's tokens went by model, by kind of work, by person or agent and per role. The windows of [accounts.md](accounts.md) count tokens for the limits and know nothing of what a call was for or who started it, so every call is also kept per day.

## What a call carries

`kind` is one of `chat`, `workflow_run`, `agency_review`, `test_run` or `background_job`. `person` is the opaque id of the member or manager who started the work and is empty when the agent started it itself. Both ride on the usage record (`ai_record_usage`, `UsageInput`), on `ai_chat_account` (`ChatInput`) and, for the seat gateway, on the headers `X-Usage-Kind` and `X-Usage-Person`.

A chat as an agent role (`role_id` given) with no kind is refused as an invalid chat, "the call names no kind of work", before the model is reached. A gateway call with no kind header, or one that is not one of the five, is refused with 400 and the same sentence before anything leaves. A kind that is not one of the five is refused on `ai_record_usage` too. A record that names no kind is still counted against every limit and is kept with an empty kind, which the breakdown reads as `other`.

## The `usage_day` object

One row for each agency, account, AI role, UTC day, model, kind and person, counting requests, tokens in and tokens out. The model is the model name the account carried when the call was made, so renaming an account's model keeps its history under the old name. The minute, day and month rows of `usage` are unchanged: the five limits count exactly as before and read nothing from `usage_day`.

## `ai_usage_breakdown`

`GET /agencies/{agencyID}/usage/breakdown`, action `ai.usage.breakdown`, tool `ai_usage_breakdown`.

| Input | |
|---|---|
| `window` (required) | `today`; `7d`, today and the six days before; `month`, the first of this month to today; `last_month`, the whole of the month before |
| `group_by` | `person` or `agent`; only says which the caller will chart, the rows always carry both |
| `role_id`, `model` | keep one agent or one model |

It answers `window`, `from`, `to` (UTC days, `YYYY-MM-DD`) and `rows`, oldest day first: `day`, `model`, `role_id`, `role` (the agent's name, empty with no agent), `kind`, `person`, `tokens_in`, `tokens_out`, `requests`. The rows are summed in the database over every account of the agency, so the breakdown is never cut at the store's list cap, and the rows of `month` add up to the monthly tokens that `ai_list_usage` counts. Another agency's use never appears.

Managers only. The module has no notion of a staff seat; the host's `Authorize` decides by action name, so the host mounting it refuses `ai.usage.breakdown` to a staff seat and the refusal is one plain line.
