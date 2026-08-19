// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// Statistics are per column chunk so a file with 3 row groups has 3 entries
// and the ranges must not overlap.
func TestStatisticsPerRowGroup(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "min" entry))) (parquet-statistics reader "id"))`,
		Expect: `(0 5 10)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "max" entry))) (parquet-statistics reader "id"))`,
		Expect: `(4 9 14)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "row-group" entry))) (parquet-statistics reader "id"))`,
		Expect: `(0 1 2)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "num-values" entry))) (parquet-statistics reader "id"))`,
		Expect: `(5 5 5)`,
	}).Test(t)
}

func TestStatisticsStringColumn(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "min" (car (parquet-statistics reader "name"))))`,
		Expect: `"row-00"`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "max" (car (parquet-statistics reader "name"))))`,
		Expect: `"row-04"`,
	}).Test(t)
}

// The null count comes from the statistics rather than from reading the data.
func TestStatisticsNullCount(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "null-count" (car (parquet-statistics reader "i64"))))`,
		Expect: "1",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "min" (car (parquet-statistics reader "i64"))))`,
		Expect: "0",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "max" (car (parquet-statistics reader "i64"))))`,
		Expect: "7",
	}).Test(t)
}

// Statistics are optional. A file without them still reports the row group and
// the value count but has no min or max at all.
func TestStatisticsAbsent(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (car (parquet-statistics reader "id")))`,
		Expect: `("row-group" "num-values")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "min" (car (parquet-statistics reader "id"))))`,
		Expect: "nil",
	}).Test(t)
}

// Leaving the column out gives every column keyed by its path.
func TestStatisticsAllColumns(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-statistics reader))`,
		Expect: `("id" "name")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (cdr (assoc "id" (parquet-statistics reader))) (parquet-statistics reader "id"))`,
		Expect: "t",
	}).Test(t)
}

// Statistics belong to the leaf columns so a nested column is named by its
// dotted path, not by the struct that holds it.
func TestStatisticsNestedPath(t *testing.T) {
	scope := reader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-statistics reader))`,
		Expect: `("i64" "f64" "str" "bool" "date" "bin" "lst.list.element" "st.a")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "max" (car (parquet-statistics reader "st.a"))))`,
		Expect: "5",
	}).Test(t)
}

func TestStatisticsByIndex(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (parquet-statistics reader 0) (parquet-statistics reader "id"))`,
		Expect: "t",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (parquet-statistics reader 1) (parquet-statistics reader "name"))`,
		Expect: "t",
	}).Test(t)
}

func TestStatisticsMethod(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (send reader :statistics "id") (parquet-statistics reader "id"))`,
		Expect: "t",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (send reader :statistics) (parquet-statistics reader))`,
		Expect: "t",
	}).Test(t)
}

func TestStatisticsBadArgs(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-statistics reader "nope")`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-statistics reader 99)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-statistics reader 1.5)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-statistics 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :statistics "id" "name")`,
		Panics: true,
	}).Test(t)
}
