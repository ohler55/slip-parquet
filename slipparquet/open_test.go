// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

func TestOpenOk(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: `(setq reader (parquet-open "testdata/primitive.parquet"))`,
		Expect: "/#<parquet-reader-flavor [0-9a-f]+>/",
	}).Test(t)
}

func TestOpenBadFile(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}

// A keyword that is not a symbol is reported as the wrong type rather than as a
// keyword with no value, which is what a bare check for a following value would
// say when the bad argument is the last one.
func TestOpenBadKeyword(t *testing.T) {
	for _, src := range []string{
		`(parquet-open "testdata/primitive.parquet" 7)`,
		`(parquet-open "testdata/primitive.parquet" 7 :string)`,
	} {
		(&sliptest.Function{
			Source:    src,
			PanicType: slip.Symbol("type-error"),
		}).Test(t)
	}
}
