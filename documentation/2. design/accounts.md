# Accounts

HS122. An account is an agency's standing with one model provider. Two objects: `account` (everything an operation may show, including `key_set`) and `account_key` (the sealed key, in a table of its own that no operation reaches).

## Agency scope

The agency id is an opaque string the host supplies in the address (`/agencies/{agencyID}/accounts`). It is bound by the address, never read from a body, and every read and write filters on it. Another agency's account answers as not found. A name is unique within one agency (a unique index on agency and name, plus a check that answers `ErrDuplicateName`).

## The key

`PUT .../key` replaces the key and answers the account with `key_set` true. Nothing returns it. The key is sealed with AES-256-GCM under a 32-byte master key passed with `WithMasterKey`; the stored value is a random nonce followed by the ciphertext, base64. With no master key `SetAccountKey` answers `ErrNoMasterKey` and stores nothing. The gateway reads a key through `KeyReader.AccountKey(ctx, agencyID, accountID)`, a Go method that no operation declares.

Tests pin that the key is absent from every operation's output, every error and the log, on both the manager and the HTTP route table, and that the stored value is not the plain key.

## Provider agnostic

`provider`, `base_url` and `model_name` are all required, with no default. A provider kind with no adapter (`HasAdapter`) is refused with `ErrUnsupportedKind` on create and on update. The first kind is `openai_compatible`.

## Not built here

The test operation, usage counting and limit enforcement are HS123 and HS124. The five limits and the last-test fields are stored only; zero means none.

## Second domain

`spec/examples/research.ai.json` names the same two roles `connection` and `credential`, so the module is exercised under words that differ from the default domain.
