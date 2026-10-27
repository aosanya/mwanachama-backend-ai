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

## Users

The Test of an account (`httpProber`) is `Call` with one short prompt. The gateway of HS127 will use `Call` with tools and read the token counts for usage.

## Tests

`adapter_test.go` runs each kind against an httptest fake: a request and tool call round trip, token counts, the key in its one header and nowhere else, status mapping, and a Test through the real prober. Each was checked to fail with the key moved into the URL, into a wrong header, and with the usage count dropped.
