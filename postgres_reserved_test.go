package ai_test

import (
	"regexp"
	"strings"
	"testing"

	ai "github.com/aosanya/mwanachama-backend-ai"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const postgresReservedWords = `
all analyse analyze and any array as asc asymmetric both case cast check
collate column constraint create current_catalog current_date current_role
current_time current_timestamp current_user default deferrable desc distinct
do else end except false fetch for foreign from grant group having in
initially intersect into lateral leading limit localtime localtimestamp not
null offset on only or order placing primary references returning select
session_user some symmetric system_user table then to trailing true union
unique user using variadic when where window with
authorization binary collation concurrently cross current_schema freeze full
ilike inner is isnull join left like natural notnull outer overlaps right
similar tablesample verbose
`

var postgresReserved = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(postgresReservedWords) {
		out[w] = true
	}
	return out
}()

type identifier struct{ kind, owner, name string }

var (
	createTableStatement = regexp.MustCompile(`(?s)^create table if not exists (\S+) \(\n(.*)\n\)$`)
	createIndexStatement = regexp.MustCompile(`^create (?:unique )?index if not exists (\S+) on (\S+) \((.*?)\)(?: where .*)?$`)
	primaryKeyClause     = regexp.MustCompile(`^primary key \((.*)\)$`)
)

func identifiersIn(t *testing.T, ddl []string) []identifier {
	t.Helper()
	var out []identifier
	for _, stmt := range ddl {
		if m := createTableStatement.FindStringSubmatch(stmt); m != nil {
			table := m[1]
			out = append(out, identifier{"table", "", table})
			for _, line := range strings.Split(m[2], "\n") {
				line = strings.TrimSuffix(strings.TrimSpace(line), ",")
				if pk := primaryKeyClause.FindStringSubmatch(line); pk != nil {
					for _, col := range strings.Split(pk[1], ",") {
						out = append(out, identifier{"primary key column", table, strings.TrimSpace(col)})
					}
					continue
				}
				out = append(out, identifier{"column", table, strings.Fields(line)[0]})
			}
			continue
		}
		if m := createIndexStatement.FindStringSubmatch(stmt); m != nil {
			out = append(out, identifier{"index", m[2], m[1]})
			if strings.HasPrefix(m[3], "(") {
				continue
			}
			for _, col := range strings.Split(m[3], ",") {
				out = append(out, identifier{"index column", m[1], strings.TrimSpace(col)})
			}
			continue
		}
		t.Fatalf("the DDL guard cannot read this statement, so it cannot vouch for it:\n%s", stmt)
	}
	return out
}

func unquotedReserved(ids []identifier) []identifier {
	var out []identifier
	for _, id := range ids {
		if strings.HasPrefix(id.name, `"`) {
			continue
		}
		if postgresReserved[strings.ToLower(id.name)] {
			out = append(out, id)
		}
	}
	return out
}

func TestPostgresDDLNamesNoReservedWord(t *testing.T) {
	for _, path := range shippedSpecs(t) {
		t.Run(path, func(t *testing.T) {
			s, err := ai.LoadSpec(path)
			if err != nil {
				t.Fatalf("LoadSpec: %v", err)
			}
			s.Instance = "test"
			for _, id := range unquotedReserved(identifiersIn(t, s.DDL("postgres"))) {
				t.Errorf("%s %q (%s) is a Postgres reserved word and the DDL leaves it unquoted: rename it in the blueprint", id.kind, id.name, id.owner)
			}
		})
	}
}

func TestPostgresDDLGuardReadsEveryTableAndColumn(t *testing.T) {
	for _, path := range shippedSpecs(t) {
		t.Run(path, func(t *testing.T) {
			s, err := ai.LoadSpec(path)
			if err != nil {
				t.Fatalf("LoadSpec: %v", err)
			}
			s.Instance = "test"
			tables, twins, columns := 0, 0, 0
			for _, id := range identifiersIn(t, s.DDL("postgres")) {
				switch id.kind {
				case "table":
					tables++
					if strings.HasSuffix(id.name, spec.TestSuffix) {
						twins++
					}
				case "column":
					columns++
				}
			}
			if twins != len(s.Objects) {
				t.Fatalf("guard read %d _test twin tables; the spec declares %d objects", twins, len(s.Objects))
			}
			wantTables, wantColumns := 2*len(s.Objects), 0
			for _, o := range s.Objects {
				wantColumns += 2*len(o.Fields) + 1
			}
			if tables != wantTables || columns != wantColumns {
				t.Fatalf("guard read %d tables and %d columns; the spec declares %d and %d", tables, columns, wantTables, wantColumns)
			}
		})
	}
}

func TestPostgresDDLGuardFlagsAReservedWordWhereverItSits(t *testing.T) {
	ddl := []string{
		"create table if not exists t_one (\n  id text not null,\n  window text not null,\n  primary key (id)\n)",
		"create table if not exists window (\n  id text not null,\n  primary key (id)\n)",
		"create table if not exists t_two (\n  id text not null,\n  group text,\n  primary key (id, user)\n)",
		"create index if not exists t_one_a_idx on t_one (id, order) where id = 'x'",
		"create unique index if not exists limit on t_one (id)",
		"create table if not exists t_three (\n  \"select\" text not null,\n  primary key (\"select\")\n)",
	}
	got := map[string]bool{}
	for _, id := range unquotedReserved(identifiersIn(t, ddl)) {
		got[id.kind+" "+id.name] = true
	}
	for _, want := range []string{
		"column window", "table window", "column group", "primary key column user",
		"index column order", "index limit",
	} {
		if !got[want] {
			t.Errorf("guard missed %q; flagged %v", want, got)
		}
	}
	if len(got) != 6 {
		t.Errorf("guard flagged %v; a quoted identifier must pass", got)
	}
}
