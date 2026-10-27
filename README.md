# mwanachama-backend-ai

Holds an agency's AI: the **accounts** it has with model providers, the **AI roles** that speak for its human roles, and the **limits** and **usage** that keep both in bounds.

An **AI role** is a container for a system prompt. It carries a name, the prompt, the AI account it uses, and the agency role it speaks for. An **AI account** is an agency's standing with one model provider: a base URL, a model name, a key that can be written but never read back, and caps on use. An agency can hold many accounts and many AI roles, and maps each role to an account.

A Go library in the declared-domain shape of `mwanachama-backend-catalog`: objects in `ai.blueprint.json`, the route table in `ai.operations.json`, storage through `mwanachama-backend-shared/specstore`. A host mounts it per agency, does identity and the permission checks, and sees only opaque member and role ids.

## Status

Not built yet. This repository holds the requirements and the house rules; the code arrives row by row. See [documentation/](documentation/README.md).

## What it will do

| Feature | In one line |
|---|---|
| AI accounts | Add, change and remove an account; set its key without ever reading it back |
| Test an account | One short request with the account's key, answered as working or one plain reason |
| Limits and usage | A daily request cap and a monthly token cap per account, counted by role and day |
| AI roles | A named system prompt, edited in place, used on the very next message |
| Role mapping | Each AI role uses one account, and speaks for one agency role |
| Model gateway | An OpenAI-compatible endpoint that adds the account's key, counts the use and stops at the cap, so the agents never hold a model key |

## Licence

Apache 2.0, see [LICENSE](LICENSE).
