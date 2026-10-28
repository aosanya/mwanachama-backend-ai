# QA

Test layout and results arrive with the first code.

HS125 tests: `airole_test.go` (live agency part, switch off, cap, lower-of-role-and-account in all five limits, one revision per edit with its editor, restore). Each was shown to fail with its fix removed.

Postgres reserved words: `postgres_reserved_test.go` (no unquoted table, column or index column in the postgres DDL is a reserved word; the guard reads every declared table and column; it flags a reserved word in a table name, column, primary key, index name and index column and lets a quoted one pass). It was shown to fail on the `window` column of `usage` before the rename to `window_kind`, in both shipped specs. `usage_window_test.go` pins the rename: the rows keep their window in `window_kind`, and the `ai_record_usage` answer keeps the key `window`.
