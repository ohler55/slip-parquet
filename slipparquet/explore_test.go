// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// explored runs parquet-explore into a string so the whole table can be
// compared at once.
func explored(t *testing.T, scope *slip.Scope, args string) string {
	t.Helper()
	v := slip.ReadString(
		`(with-output-to-string (s) (parquet-explore reader `+args+` :stream s))`, scope).Eval(scope, nil)
	str, _ := v.(slip.String)

	return string(str)
}

func TestExploreMarkdown(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, `| id | name   |
| -- | ------ |
| 0  | row-00 |
| 1  | row-01 |
| 2  | row-02 |
`, explored(t, scope, ":limit 3"))
}

// The default format can also be asked for by name.
func TestExploreMarkdownExplicit(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, explored(t, scope, ":limit 2"), explored(t, scope, ":limit 2 :format :markdown"))
}

func TestExploreTSV(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, "id\tname\n0\trow-00\n1\trow-01\n", explored(t, scope, ":limit 2 :format :tsv"))
}

func TestExploreColumnsAndPaging(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, `| id |
| -- |
| 5  |
| 6  |
`, explored(t, scope, `'("id") :offset 5 :limit 2`))
	// A symbol and an index name a column just as a string does.
	tt.Equal(t, explored(t, scope, `'("id") :limit 1`), explored(t, scope, `'(0) :limit 1`))
}

// The default is 10 rows.
func TestExploreDefaultLimit(t *testing.T) {
	scope := reader(t, "big.parquet")
	// A header, a rule, and 10 rows.
	tt.Equal(t, 12, len(splitLines(explored(t, scope, ""))))
}

func splitLines(s string) (lines []string) {
	for _, ln := range []byte(s) {
		_ = ln
	}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return
}

// A long value is cut short so a column of text stays readable, and every line
// of the table is the same width.
func TestExploreWidth(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	out := explored(t, scope, ":limit 2 :width 4")
	tt.Equal(t, `| id | name |
| -- | ---- |
| 0  | row… |
| 1  | row… |
`, out)
	for _, ln := range splitLines(out) {
		tt.Equal(t, 13, len([]rune(ln)), ln)
	}
	// A zero width leaves values whole.
	tt.Equal(t, `| id | name   |
| -- | ------ |
| 0  | row-00 |
`, explored(t, scope, ":limit 1 :width 0"))
}

// A null is an empty cell by default which makes it look like an empty string.
// The :null option tells them apart.
func TestExploreNull(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	tt.Equal(t, `| i64 | str | lst   |
| --- | --- | ----- |
| 7   | a   | (1 2) |
|     |     |       |
| 0   |     | nil   |
`, explored(t, scope, `'("i64" "str" "lst")`))

	tt.Equal(t, `| i64  | str  | lst   |
| ---- | ---- | ----- |
| 7    | a    | (1 2) |
| NULL | NULL | NULL  |
| 0    |      | nil   |
`, explored(t, scope, `'("i64" "str" "lst") :null "NULL"`))
}

func TestExploreSummary(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, "15 rows, 2 columns, 3 row groups, showing 1-3",
		splitLines(explored(t, scope, ":limit 3 :summary t"))[0])
	// A blank line separates the summary from the table.
	tt.Equal(t, "", splitLines(explored(t, scope, ":limit 3 :summary t"))[1])
	// Off by default so the output stays pasteable.
	tt.Equal(t, "|", explored(t, scope, ":limit 1")[:1])
}

func TestExploreSummarySingular(t *testing.T) {
	scope := reader(t, "shard-0.parquet")
	tt.Equal(t, "5 rows, 2 columns, 1 row group, showing 1-1",
		splitLines(explored(t, scope, ":limit 1 :summary t"))[0])
}

func TestExploreSummaryNoRows(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	tt.Equal(t, "15 rows, 2 columns, 3 row groups, showing none",
		splitLines(explored(t, scope, ":offset 99 :summary t"))[0])
}

// A dataset says how many shards it is made of.
func TestExploreSummaryShards(t *testing.T) {
	scope := dataset(t)
	tt.Equal(t, "15 rows in 3 shards, 2 columns, 3 row groups, showing 5-7",
		splitLines(explored(t, scope, ":offset 4 :limit 3 :summary t"))[0])
}

// Nothing is written past the rows asked for even when the file is huge.
func TestExploreReadsOnlyWhatItShows(t *testing.T) {
	scope := reader(t, "big.parquet")
	out := explored(t, scope, ":offset 9998 :limit 5")
	tt.Equal(t, `| id   |
| ---- |
| 9998 |
| 9999 |
`, out)
}

// Newlines and tabs in a value would break the table apart.
func TestExploreFlattensWhitespace(t *testing.T) {
	scope := reader(t, "binary.parquet")
	out := explored(t, scope, ":limit 3")
	for _, ln := range splitLines(out) {
		tt.Equal(t, false, containsAny(ln, "\t\n\r"), ln)
	}
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, r := range s {
			if r == c {
				return true
			}
		}
	}
	return false
}

func TestExploreBadArgs(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	for _, src := range []string{
		`(parquet-explore reader :format :csv)`,
		`(parquet-explore reader :limit "two")`,
		`(parquet-explore reader :offset -1)`,
		`(parquet-explore reader :width "wide")`,
		`(parquet-explore reader :null 7)`,
		`(parquet-explore reader :bogus 1)`,
		`(parquet-explore reader :limit)`,
		`(parquet-explore reader :stream 7)`,
		`(parquet-explore reader '("nope"))`,
		`(parquet-explore reader '("id") 7 1)`,
	} {
		(&sliptest.Function{Scope: scope, Source: src, Panics: true}).Test(t)
	}
	(&sliptest.Function{
		Source:    `(parquet-explore 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}

// Explore writes rather than returns, so it hands back no value at all in the
// way describe does.
func TestExploreReturnsNovalue(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	v := slip.ReadString(
		`(with-output-to-string (s) (parquet-explore reader :limit 1 :stream s))`, scope).Eval(scope, nil)
	str, _ := v.(slip.String)
	tt.Equal(t, 3, len(splitLines(string(str))), "header, rule, and one row")

	// With no stream it goes to *standard-output*.
	out := slip.ReadString(
		`(with-output-to-string (s) (let ((*standard-output* s)) (parquet-explore reader :limit 1)))`,
		scope).Eval(scope, nil)
	os, _ := out.(slip.String)
	tt.Equal(t, 3, len(splitLines(string(os))), "the table went to *standard-output*")
}
