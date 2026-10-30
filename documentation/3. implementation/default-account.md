# The default account (HS331)

A hub marks one of its AI accounts as the default. A test run spends from it, so the Test agent asks the module for it before the run starts.

## How it is stored

The account object gains `is_default` (bool, false until marked). The wire name is `default`, so `ai_list_accounts` and `ai_get_account` show `"default": true` on the marked account and `false` on the rest. The column is `is_default` because `default` is a Postgres reserved word and the DDL guard refuses it unquoted.

There is no second table and no new object, so the DDL guard and the domain specs need no entry: the guard counts the new column on its own, and `spec.Migrate` adds it to an existing `accounts` table.

## Marking

`ai_set_default_account` (`PUT /agencies/{agencyID}/accounts/{accountID}/default`, action `ai.account.default_set`) takes `agencyID` and `accountID` and no body. It clears the mark on the agency's other accounts, then sets it on the named one, so a second mark moves the default and an agency never holds two. An account that is not in the agency, or not there at all, is refused with `ErrAccountNotFound` (404) and the default stays where it was.

The mark belongs to this operation alone:

- A new account is never the default, whatever `default` its body carries.
- Changing an account's settings, limits or key leaves the mark as it was; a `default` in an update body is ignored.
- Removing the default account leaves none marked.

## Reading

`ai_get_default_account` (`GET /agencies/{agencyID}/default-account`, action `ai.account.default_read`) takes `agencyID` and returns the account. With none marked it answers `ErrNoDefaultAccount` (404), whose message is the plain sentence "Mark a default AI account under Models before running a test". The host shows that sentence as it is.
