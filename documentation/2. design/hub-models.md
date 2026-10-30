# A hub holds several models (HS203, decisions 64 and 65 of Hub Steward)

A hub's models are its AI accounts. Adding one never replaces another: `ai_create_account` mints a new row, a name is unique within the agency, and the model name is not, so the same model reached through two providers is two accounts. Each keeps its own provider, address, model name, sealed key and limits. `hub_models_test.go` pins that a second account leaves the first with its key, limits and chat switch.

## Which model an agent uses

An AI role maps to at most one account (`ai_map_role`). `ai_resolve_role` and `ai_resolve_agency_role` answer, in order:

1. the role's own account, `account_source` `role`;
2. the agency's default account (`ai_set_default_account`), `account_source` `default`;
3. neither: the line "no account is mapped", no account and no limits.

The limits in force follow the account that answered. Another agency's default is never used. The gateway resolves through the same call, so it follows the same order.

## Removing a model that agents use

`ai_delete_account` clears the account from every AI role of the agency that was mapped to it, leaving the roles, their prompts and their agency role mapping as they were. Those roles then use the default account, or answer "no account is mapped" when none is marked. The removal is not refused and does not ask. Removing the default account leaves none marked.

This is the module's choice where the board row named the question and no decision answered it: a refusal would block "Remove this model" until every agent was moved by hand. Refusing instead is a change to `DeleteAccount` alone.
