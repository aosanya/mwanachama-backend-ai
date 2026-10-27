# Requirements

## Why this module exists

An agency's AI is spread across places that do not know each other: a model key typed into one person's browser, a system prompt written into agent source code, a key in an environment variable on one machine. None of them can be shared between agents, capped, tested or changed by the person who runs the agency. This module is the one home for all of it.

## The objects

| Object | What it holds |
|---|---|
| Account | An agency's standing with one model provider: name, provider, base URL, model name, status, a flag saying whether a key is set, the last test result, and its five limits |
| Account key | The key itself, in its own object that no operation reaches, sealed at rest |
| AI role | A container for a system prompt: name, description, the manager's fine-tuning, a switch for the agency's instructions, status, the account it uses, the agency role it speaks for, and its own five limits |
| Usage | Requests and tokens in and out, counted by account, AI role and day. This is where consumption is stored: in this module's own `usage` object, in the database the host gives it, keyed by agency like the other modules' data. The one-minute rate may use a small in-memory window that resets on a restart; every other limit counts from the stored rows |

## What a person can do

- Add several accounts to an agency and change or remove any of them.
- Set an account's key. It is never shown again, only whether it is set.
- Test an account: one short request, answered as working or as one of four plain reasons (no key is set, the model refused the key, the model could not be reached, the model answered with an error).
- Set five limits on an account and on each AI role: tokens per call, tokens per day, tokens per month, requests per day and requests per minute. Zero means none. For each limit the lower of the role's and the account's applies. Tokens per call cuts a reply short; the others refuse the call, in one plain sentence that names whose limit and which.
- See use against each limit, by account and by AI role.
- Create AI roles, each a container for a system prompt.
- Map each AI role to an account and link it to the agency role it speaks for.
- Fine-tune a role's instructions. Most of them come from the agency (see below); the manager adds fine-tuning of their own, sees the two together as one preview, and can switch the agency's instructions off so the role runs on the fine-tuning alone. The next message uses the result.

## Where the instructions come from

The agency already declares what a seat needs: the seat's description, the objectives it is answerable for through RACI, the values and rules attached to it or to the whole organization, and the instruction on each work item that serves those objectives. The agency module composes these into a brief for a seat. This module asks the host for that brief on every resolve, through an instruction source, and never copies it, so a change in the agency reaches the agent by itself. Only the manager's fine-tuning is stored here.

## What an agent can do

- Ask which AI role it is acting as and receive that role's composed instructions, with their parts, and its account details and limits in force, without the key.
- Send its model calls through the module's OpenAI-compatible endpoint. The module picks the account from the agent's own agency role, adds the key, forwards the call, lowers the reply length to the tokens-per-call limit, counts the use and refuses at any other limit. The agent never holds a model key.

## Never

- Return, print or log an account's key, in any operation, error or test failure.
- Hold a model call past a limit. A refusal says which limit in one plain sentence.
- Name a domain. The module sees opaque ids.

## Out of scope for now

Streaming responses, per-member limits, billing, key rotation schedules, provider-specific features beyond an OpenAI-compatible chat completion.
