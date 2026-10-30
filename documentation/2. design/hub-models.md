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

## The chat box's model picker (HS189, decision 60)

The flag `chat_enabled` and the label `chat_label` live on the account (see `accounts.md`). Two operations serve the picker, so the console does not filter the accounts itself:

- `ai_list_chat_models` (`GET /agencies/{agencyID}/chat-models`, action `ai.chat_model.list`) answers `[{id, label, picked}]`: the agency's accounts that are `enabled` and `chat_enabled`, in the Models page's order (creation order). `label` is `chat_label`, or `model_name` when no label was typed. Exactly one item is `picked` when the list is not empty: the caller's own pick if that account is still on the list, otherwise the first. An agency with none answers `[]`. The item carries no key, no address and no limits.
- `ai_pick_chat_model` (`PUT /agencies/{agencyID}/chat-models/pick`, action `ai.chat_model.pick`) takes `{account_id}` and answers the same list with the pick marked. A model the chat may not use answers `ErrChatModelUnavailable` (409), another agency's or a missing one `ErrAccountNotFound` (404), and a call with no signed-in person 400.

The pick is kept per person: the object `chat_pick`, one row per agency and person, stored under the `person_id` argument that the mount supplies from `Caller` (never read from the request, as for `saved_by`). A pick of a model that is later switched off for the chat, or disabled, is not removed; the list simply falls back to the first model until it is allowed again. Removing an account removes the picks of it. A new table needs no migration for an existing database: the shared migration creates it.

The last model that can be used is not protected here: unticking it is accepted and the list then answers `[]`, as the module's rule at "Chat use" in `accounts.md` has always said. The console owns the rule in decision 60 that a hub's only chat model stays ticked.
