// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// reader opens a testdata file and arranges for it to be closed.
func reader(t *testing.T, name string) *slip.Scope {
	t.Helper()
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(`(setq reader (parquet-open "testdata/`+name+`"))`, scope).Eval(scope, nil)
	t.Cleanup(func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) })

	return scope
}

func TestRowGroupCountOk(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-row-group-count reader)`,
		Expect: "1",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-group-count)`,
		Expect: "1",
	}).Test(t)
}

func TestRowGroupCountBadReader(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-row-group-count 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-row-group-count (make-instance 'vanilla-flavor))`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}

func TestRowGroupInfoOk(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	// The byte sizes are file specific so check the shape and the row count
	// which are the parts paging depends on.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "rows" (parquet-row-group-info reader 0)))`,
		Expect: "8",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "columns" (parquet-row-group-info reader 0)))`,
		Expect: "11",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-row-group-info reader 0))`,
		Expect: `("shard" "rows" "columns" "byte-size" "compressed-size")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(< 0 (cdr (assoc "byte-size" (parquet-row-group-info reader 0))))`,
		Expect: "t",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "rows" (send reader :row-group-info 0)))`,
		Expect: "8",
	}).Test(t)
}

// The row counts of every row group have to add up to the file row count since
// paging relies on that to skip groups.
func TestRowGroupRowsSumToRowCount(t *testing.T) {
	for _, name := range []string{"primitive.parquet", "binary.parquet", "nested-struct.parquet"} {
		scope := reader(t, name)
		(&sliptest.Function{
			Scope: scope,
			Source: `(let ((total 0))
                       (dotimes (i (parquet-row-group-count reader))
                         (setq total (+ total (cdr (assoc "rows" (parquet-row-group-info reader i))))))
                       (= total (parquet-row-count reader)))`,
			Expect: "t",
		}).Test(t)
	}
}

func TestRowGroupInfoOutOfRange(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-row-group-info reader 1)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-row-group-info reader -1)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(send reader :row-group-info 99)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
}

func TestRowGroupInfoBadArgs(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-row-group-info reader "zero")`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-group-info)`,
		Panics: true,
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-row-group-info 7 0)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}
