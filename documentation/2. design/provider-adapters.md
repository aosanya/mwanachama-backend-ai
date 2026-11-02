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

## Users

The Test of an account (`httpProber`) is `Call` with one short prompt. The gateway uses `Call` for an `anthropic` account (HS624), reading the token counts from its `Reply` for usage; an `openai_compatible` account is still forwarded as the agent sent it.

## Model lists

Each kind also lists its models (`GET` per kind, see [the model list](models.md)) through `ModelLister`, kept apart from `Adapter` so an adapter written elsewhere need not grow a method.

## Tests

`adapter_test.go` runs each kind against an httptest fake: a request and tool call round trip, token counts, the key in its one header and nowhere else, status mapping, and a Test through the real prober. Each was checked to fail with the key moved into the URL, into a wrong header, and with the usage count dropped.
