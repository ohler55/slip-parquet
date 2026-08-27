// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"fmt"
	"testing"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// The three shard fixtures hold exactly the same 15 rows as
// multi-group.parquet so a dataset can be compared against a single file.
const shardList = `'("testdata/shard-0.parquet" "testdata/shard-1.parquet" "testdata/shard-2.parquet")`

// dataset opens the three shards as one reader.
func dataset(t *testing.T) *slip.Scope {
	t.Helper()
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(`(setq reader (parquet-open `+shardList+`))`, scope).Eval(scope, nil)
	t.Cleanup(func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) })

	return scope
}

func TestShardsRowCount(t *testing.T) {
	scope := dataset(t)
	// Summed across the shards, not taken from the first.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-row-count reader)`,
		Expect: "15",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-row-group-count reader)`,
		Expect: "3",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-column-count reader)`,
		Expect: "2",
	}).Test(t)
}

// A single file opened as a one item list has to behave exactly as the plain
// string form does.
func TestShardsSingleAsList(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (parquet-open '("testdata/multi-group.parquet")))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-row-count reader)`,
		Expect: "15",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :filepath)`,
		Expect: `"testdata/multi-group.parquet"`,
	}).Test(t)
}

func TestShardsFilepath(t *testing.T) {
	scope := dataset(t)
	// The printed form wraps so check the shape and the parts.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(length (send reader :filepath))`,
		Expect: "3",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 1 (send reader :filepath))`,
		Expect: `"testdata/shard-1.parquet"`,
	}).Test(t)
}

// The whole point: three shards read as one table produce the same rows as the
// single file holding those rows.
func TestShardsRowsMatchSingleFile(t *testing.T) {
	scope := dataset(t)
	scope.Let("one", nil)
	_ = slip.ReadString(`(setq one (parquet-open "testdata/multi-group.parquet"))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send one :close)`, scope).Eval(scope, nil) }()

	for _, src := range []string{
		`(parquet-rows %s :list)`,
		`(parquet-rows %s :assoc)`,
		`(parquet-rows %s :list '("name"))`,
		`(parquet-column %s "id")`,
		`(parquet-columns %s)`,
	} {
		(&sliptest.Function{
			Scope:  scope,
			Source: fmt.Sprintf(`(equal `+src+` `+src+`)`, "reader", "one"),
			Expect: "t",
		}).Test(t)
	}
}

// Every offset and limit has to line up with the single file too, including
// the ones that straddle a shard boundary.
func TestShardsPagingMatchesSingleFile(t *testing.T) {
	scope := dataset(t)
	scope.Let("one", nil)
	_ = slip.ReadString(`(setq one (parquet-open "testdata/multi-group.parquet"))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send one :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope: scope,
		Source: `(let ((bad nil))
                   (dotimes (off 17)
                     (dotimes (lim 17)
                       (let ((a (parquet-rows reader :list nil :offset off :limit lim))
                             (b (parquet-rows one :list nil :offset off :limit lim)))
                         (unless (equal a b) (setq bad (add bad (list off lim a b)))))))
                   bad)`,
		Expect: "nil",
	}).Test(t)
}

// A read that starts in one shard and ends in another.
func TestShardsSpanBoundary(t *testing.T) {
	scope := dataset(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 3 :limit 4)`,
		Expect: `((3) (4) (5) (6))`,
	}).Test(t)
	// Across all three shards.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 4 :limit 7)`,
		Expect: `((4) (5) (6) (7) (8) (9) (10))`,
	}).Test(t)
	// Entirely inside the last shard.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 12 :limit 2)`,
		Expect: `((12) (13))`,
	}).Test(t)
	// Past the end of the dataset.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 15)`,
		Expect: "nil",
	}).Test(t)
}

func TestShardsEachRow(t *testing.T) {
	scope := dataset(t)
	scope.Let("seen", nil)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (parquet-each-row reader (lambda (row) (setq seen (add seen (car row)))) :list '("id"))
                   seen)`,
		Expect: `(0 1 2 3 4 5 6 7 8 9 10 11 12 13 14)`,
	}).Test(t)
}

// Row groups are numbered across the whole dataset and each reports the shard
// it came from.
func TestShardsRowGroupInfo(t *testing.T) {
	scope := dataset(t)
	for _, want := range []string{"0", "1", "2"} {
		(&sliptest.Function{
			Scope:  scope,
			Source: `(cdr (assoc "shard" (parquet-row-group-info reader ` + want + `)))`,
			Expect: want,
		}).Test(t)
	}
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "rows" (parquet-row-group-info reader 2)))`,
		Expect: "5",
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-row-group-info reader 3)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
}

// Statistics span the shards with a global row group number.
func TestShardsStatistics(t *testing.T) {
	scope := dataset(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "min" entry))) (parquet-statistics reader "id"))`,
		Expect: `(0 5 10)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "row-group" entry))) (parquet-statistics reader "id"))`,
		Expect: `(0 1 2)`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (entry) (cdr (assoc "shard" entry))) (parquet-statistics reader "id"))`,
		Expect: `(0 1 2)`,
	}).Test(t)
}

// The schema and the file level facts come from the first shard.
func TestShardsSchema(t *testing.T) {
	scope := dataset(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (f) (send f :name)) (send (send reader :schema) :fields))`,
		Expect: `("id" "name")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (parquet-created-by reader))`,
		Expect: "t",
	}).Test(t)
}

// Shards that do not agree on a schema can not be read as one table.
func TestShardsSchemaMismatch(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open '("testdata/shard-0.parquet" "testdata/shard-odd.parquet"))`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
}

func TestShardsBadArgs(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open '())`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open '("testdata/shard-0.parquet" 7))`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open '("testdata/shard-0.parquet" "testdata/nope.parquet"))`,
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}

// make-instance takes a list of files too.
func TestShardsMakeInstance(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (make-instance 'parquet-reader-flavor :file `+shardList+`))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-count)`,
		Expect: "15",
	}).Test(t)
}

// Shards that differ in how many columns they have are caught the same way as
// ones that differ in column names.
func TestShardsFieldCountMismatch(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open '("testdata/shard-0.parquet" "testdata/primitive.parquet"))`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
}
