// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip/sliptest"
)

// The README examples use a file shaped like multi-group.parquet. Running them
// here keeps the documented output from drifting away from the code.
func TestReadmeExamples(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	for _, ex := range []struct{ src, want string }{
		{`(send reader :row-count)`, "15"},
		{`(send reader :column-count)`, "2"},
		{`(send reader :created-by)`, `"parquet-go version 18.7.0"`},
		{`(mapcar (lambda (f) (send f :name)) (send (send reader :schema) :fields))`, `("id" "name")`},
		{`(send reader :rows :list :limit 3)`, `((0 "row-00") (1 "row-01") (2 "row-02"))`},
		{`(send reader :rows :assoc '("id") :offset 10 :limit 2)`, `((("id" . 10)) (("id" . 11)))`},
		{`(send reader :row-group-count)`, "3"},
		{`(send reader :row-group-info 1)`,
			`(("rows" . 5) ("columns" . 2) ("byte-size" . 188) ("compressed-size" . 188))`},
		// The printed form of this one wraps so check the keys and values.
		{`(mapcar 'car (car (send reader :statistics "id")))`,
			`("row-group" "num-values" "min" "max" "null-count" "distinct-count")`},
		{`(mapcar 'cdr (car (send reader :statistics "id")))`, `(0 5 0 4 0 nil)`},
		{`(equal (parquet-rows reader :list) (send reader :rows :list))`, "t"},
	} {
		(&sliptest.Function{Scope: scope, Source: ex.src, Expect: ex.want}).Test(t)
	}
}

// The early exit example from the README has to work as written.
func TestReadmeEarlyExit(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	scope.Let("seen", nil)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (block walk
                     (send reader :each-row
                           (lambda (row) (when (< 3 (car row)) (return-from walk))
                                         (setq seen (add seen (car row))))
                           :list '("id")))
                   seen)`,
		Expect: `(0 1 2 3)`,
	}).Test(t)
}
