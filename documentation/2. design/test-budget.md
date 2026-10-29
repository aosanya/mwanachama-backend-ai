# Tokens per test and Scenarios at once (HS281)

An account and an AI role each hold nine limits: the seven of [weekly-limits.md](weekly-limits.md) and, since HS281, tokens per test and scenarios at once. Decision 82 (f) of the Hub Steward requirements names them; the Models limits page shows them as two more rows in the same form.

## Both are nullable, like the rest

Null (blank on the page) means the limit does not apply and zero means none; both are unenforced, as for the other seven. Null reads back as null and zero as zero. A negative value is refused on an account and on an AI role. Changing either on an AI role is a revision, and a restore brings it back.

The value in force is the lower non-zero of the account's and the AI role's, as for tokens per call. Every decision from `limits.check` carries both in force as `tokens_per_test` and `scenarios_at_once`, zero when no limit applies, so the host that starts the runs reads how many may go at once from the same answer.

## A run is named by its caller

The module does not own scenarios or runs; the agency backend does. A caller that makes a call for a test run adds `test_run_id` (an opaque string) and, on `usage.record`, a `step` (also opaque) to the `usage.record` and `limits.check` bodies.

- `usage.record` counts the call in the account's windows exactly as any call, so a test run spends the hub's one token pool with the agents (an agent's daily, weekly and monthly limits see it), and also adds its requests and tokens to a `test_run_usage` row keyed by agency, run, step, account and AI role. The row is named `test_run_id`, not `run_id`, because the shared engine reserves `run_id` on the `_test` twin tables.
- `limits.check` with a `test_run_id` first sums the run's tokens, in and out, over every step, account and AI role. Once the sum has reached tokens per test the answer is refused with `budget_reached` true and the sentence "this test's tokens per test limit is reached"; the host marks the run Stopped with Budget reached. The call that crosses the limit is let through and the next is the first refused, as for every other limit. Nothing already recorded is undone, so the steps done stay with their tokens. The pool limits are checked after, so a run that also hit the pool is told its budget first.
- `test-runs.usage` (`ai_list_test_run_usage`, `GET /agencies/{agencyID}/test-runs/{testRunID}/usage`) returns the run's tokens step by step and in total, with requests. A run in another agency, or one that never recorded a call, reads as empty.

A call with no `test_run_id` has no tokens-per-test budget.
