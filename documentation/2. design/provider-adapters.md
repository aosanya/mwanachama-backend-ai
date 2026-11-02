# Provider adapters (HS133)

The module names no provider. An account carries its provider kind, address and model name, and the kind picks an adapter. A kind with no adapter is refused when the account is saved (`HasAdapter`).

## Kinds

| Kind | Request | Key travels in |
| --- | --- | --- |
| `openai_compatible` | `POST {base}/chat/completions` | `Authorization: Bearer` |
| `anthropic` | `POST {base}/v1/messages`, with `anthropic-version` | `x-api-key` |
| `gemini` | `POST {base}/v1beta/models/{model}:generateContent` | `x-goog-api-key` |

The key never appears in a URL or a body. Gemini accepts it as a query parameter; this module never uses that form because URLs reach logs and error strings.

## Interface

`Adapter` has two methods: `BuildRequest` (account, key and a provider-neutral `Request` of prompt, tools and max tokens become an `*http.Request`) and `ParseReply` (the body becomes a `Reply` of text, tool calls and input and output token counts). `Call` runs one request through the adapter for the account's kind and answers a `ProbeOutcome`: 401 and 403 are refused, other non-2xx and unparseable bodies are an error, a transport failure is unreached. No outcome carries the response body, so a provider echoing a key cannot reach a stored result.

A request that carries its key in a header of its own (`x-api-key`, `x-goog-api-key`) never follows a redirect. Go strips only `Authorization`, `Cookie` and `WWW-Authenticate` when a redirect leaves the host, so a base URL that answered `3xx` toward another host would have been sent the key there. `send` therefore stops at the first response for these kinds and answers `ProbeRedirected`, for every caller that goes through it (`Call` and the model lists). The Test, the console chat and the model list read it as any other error ("the model answered with an error"); the gateway answers 502 with "the model's address redirected the request, which is not followed". An `openai_compatible` request keeps the client's default, since its key travels in `Authorization`, which Go already drops on a cross-host redirect.

Claude refuses what the other kinds accept, so its adapter shapes the body: `input_schema` always carries `type` `object` (a tool with `parameters` of `{}` or without a `type` is sent with it added, the caller's map left as it was), and an assistant turn never carries a whitespace-only text block, since Claude rejects a block with no non-whitespace text.

## What the gateway refuses before it calls Claude

For an `anthropic` account the gateway reads the agent's OpenAI chat body itself (`gateway_claude.go`). A body Claude would refuse is answered 400 with one plain sentence, nothing leaves for Claude and nothing is counted. The sentences for a body that does not decode, has no messages, carries a part that is not text, has only a system prompt or has tool-call `arguments` that are not a JSON object are in [ai-roles.md](ai-roles.md). These five are for tools and tool calls, which Claude refuses when they have no name, no id or a kind it does not know:

| Sentence | When |
| --- | --- |
| `only function tools can be carried to this model` | A tool has a `type` other than `function` (`custom`, `web_search`, `mcp` and the rest). A tool with no `type` and a `function` is sent as a function tool. |
| `a tool has no name` | A tool's `function.name` is missing, empty or only spaces, or the tool has no `function`. |
| `a tool call has no id` | A tool call of an assistant message has an `id` that is missing, empty or only spaces. |
| `a tool call has no name` | A tool call's `function.name` is missing, empty or only spaces, or the call has no `function`. |
| `a tool message has no tool_call_id` | A message with the role `tool` has a `tool_call_id` that is missing, empty or only spaces. |

The messages are read before the tools, in order, and the first problem found is the one answered; within a tool call the id is checked before the name. The console chat refuses the same cases in its own words (`chat_check.go`).

## Ignored on purpose: `functions` and `function_call`

OpenAI's deprecated `functions` (a list on the request) and `function_call` (the choice on the request, and the call on an assistant message) are not read for an `anthropic` account and are not translated into tools or tool-use blocks. The request goes out without them: no `tools` come from `functions`, and an assistant message keeps its text but gets no tool-use block from its `function_call`. An assistant message that has nothing but a `function_call` carries nothing and is left out like any other empty turn. A message with the role `function` is still carried, as a user message that begins with the function's name. An agent that needs Claude to call a tool sends `tools` and `tool_calls`. An `openai_compatible` account is forwarded the body as the agent sent it, so it receives these fields as sent.

## Users

The Test of an account (`httpProber`) is `Call` with one short prompt. The gateway uses `Call` for an `anthropic` account (HS624), reading the token counts from its `Reply` for usage; an `openai_compatible` account is still forwarded as the agent sent it.

## Model lists

Each kind also lists its models (`GET` per kind, see [the model list](models.md)) through `ModelLister`, kept apart from `Adapter` so an adapter written elsewhere need not grow a method.

## Tests

`adapter_test.go` runs each kind against an httptest fake: a request and tool call round trip, token counts, the key in its one header and nowhere else, status mapping, and a Test through the real prober. Each was checked to fail with the key moved into the URL, into a wrong header, and with the usage count dropped.

`gateway_claude_names_test.go` drives the gateway against a fake Claude: each sentence above for each way of leaving the name, id or type wrong (nothing reaches Claude), a tool with no type but a function still sent, and the deprecated fields going out without a trace. Each refusal was checked to fail with its one check removed, and the deprecated-fields test with `functions`, and then `function_call`, translated.
