# Requirements

## Why this module exists

An agency's AI is spread across places that do not know each other: a model key typed into one person's browser, a system prompt written into agent source code, a key in an environment variable on one machine. None of them can be shared between agents, capped, tested or changed by the person who runs the agency. This module is the one home for all of it.

## The objects

| Object | What it holds |
|---|---|
| Account | An agency's standing with one model provider: name, provider, base URL, model name, status, a flag saying whether a key is set, the last test result, and its limits |
| Account key | The key itself, in its own object that no operation reaches, sealed at rest |
| AI role | A container for a system prompt: name, description, the prompt, status, the account it uses and the agency role it speaks for |
| Usage | Requests and tokens in and out, counted by account, AI role and day |

## What a person can do

- Add several accounts to an agency and change or remove any of them.
- Set an account's key. It is never shown again, only whether it is set.
- Test an account: one short request, answered as working or as one of four plain reasons (no key is set, the model refused the key, the model could not be reached, the model answered with an error).
- Set limits on an account: a daily request cap and a monthly token cap. Zero means none.
- See usage against the limits.
- Create AI roles, each a system prompt. Edit the prompt in place; the next message uses it.
- Map each AI role to an account and link it to the agency role it speaks for.

## What an agent can do

- Ask which AI role it is acting as and receive that role's system prompt and account details, without the key.
- Send its model calls through the module's OpenAI-compatible endpoint. The module picks the account from the agent's own agency role, adds the key, forwards the call, counts the use and refuses at a limit. The agent never holds a model key.

## Never

- Return, print or log an account's key, in any operation, error or test failure.
- Hold a model call past a limit. A refusal says which limit in one plain sentence.
- Name a domain. The module sees opaque ids.

## Out of scope for now

Streaming responses, per-member limits, billing, key rotation schedules, provider-specific features beyond an OpenAI-compatible chat completion.
