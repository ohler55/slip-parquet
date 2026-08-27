// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip/sliptest"
)

// nulls.parquet has three rows in every column. Row 0 holds a value, row 1 is
// null, and row 2 holds the natural zero value of the type. A null and a zero
// have to be distinguishable.
func nullRows(t *testing.T, col string) *sliptest.Function {
	t.Helper()
	return &sliptest.Function{
		Scope:  reader(t, "nulls.parquet"),
		Source: `(parquet-rows reader :list '("` + col + `"))`,
	}
}

func TestNullInt64(t *testing.T) {
	f := nullRows(t, "i64")
	f.Expect = `((7) (nil) (0))`
	f.Test(t)
}

func TestNullFloat64(t *testing.T) {
	f := nullRows(t, "f64")
	f.Expect = `((1.5) (nil) (0))`
	f.Test(t)
}

func TestNullString(t *testing.T) {
	f := nullRows(t, "str")
	f.Expect = `(("a") (nil) (""))`
	f.Test(t)
}

// A null date used to read as the epoch which is a real date and so a value
// that was never in the file.
func TestNullDate(t *testing.T) {
	f := nullRows(t, "date")
	f.Expect = `((@2022-01-08T00:00:00Z) (nil) (@1970-01-01T00:00:00Z))`
	f.Test(t)
}

// A null boolean and a false boolean both read as nil. That is inherent to
// lisp where nil is false and is not something this package can change.
func TestNullBoolean(t *testing.T) {
	f := nullRows(t, "bool")
	f.Expect = `((t) (nil) (nil))`
	f.Test(t)
}

func TestNullList(t *testing.T) {
	f := nullRows(t, "lst")
	f.Expect = `(((1 2)) (nil) (()))`
	f.Test(t)
}

// A null struct is nil while a struct holding a null member keeps the member.
func TestNullStruct(t *testing.T) {
	f := nullRows(t, "st")
	f.Expect = `(((("a" . 5))) (nil) ((("a" . nil))))`
	f.Test(t)
}

// The distinction has to survive every path that produces values, not just
// :rows.
func TestNullThroughColumn(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-column reader "i64")`,
		Expect: `(7 nil 0)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 1 (parquet-column reader "str"))`,
		Expect: "nil",
	}).Test(t)
}

func TestNullThroughEachRow(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	scope.Let("seen", nil)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (parquet-each-row reader (lambda (row) (setq seen (add seen (car row))))
                                     :list '("i64"))
                   seen)`,
		Expect: `(7 nil 0)`,
	}).Test(t)
}

func TestNullThroughAssoc(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "i64" (nth 1 (parquet-rows reader :assoc '("i64")))))`,
		Expect: "nil",
	}).Test(t)
	// Row 2 is a real zero so it must not be nil.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "i64" (nth 2 (parquet-rows reader :assoc '("i64")))))`,
		Expect: "0",
	}).Test(t)
}

// Paging over a file with nulls keeps the nulls in the right rows.
func TestNullWithPaging(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("i64") :offset 1 :limit 1)`,
		Expect: `((nil))`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("i64") :offset 2 :limit 1)`,
		Expect: `((0))`,
	}).Test(t)
}
