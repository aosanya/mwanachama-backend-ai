# Weekly limits and limits that do not apply

An account and an AI role each hold seven limits: tokens per call, requests per minute, requests per day, tokens per day, tokens per month, and, since HS172, tokens per week and requests per week.

## A limit can be unset

Every limit is a nullable column. Null means the limit does not apply and zero means none; both are unenforced. A role can therefore hold only a weekly limit by leaving the other six null. `ai_set_limits` accepts null for any of them, and a stored null reads back as null, not zero.

## The weekly window is rolling

A week is the current day and the six days before it, counted from the day buckets already recorded after each call. No weekly bucket is written, so nothing is added to the record path. A call recorded on day 1 stops counting at the start of day 8. The window moves at day granularity, not by the minute: a call made late on day 1 leaves the count at midnight UTC of day 8, not seven full days later.

A calendar week was rejected because a limit would reset for everyone at the same instant, and a Monday-morning burst would be allowed twice across a boundary.

## Refusals

A refusal names the owner and the limit in one sentence, for example "this AI role's weekly token limit is reached".
