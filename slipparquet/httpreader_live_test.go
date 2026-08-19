// Copyright (c) 2026, Peter Ohler, All rights reserved.

//go:build hflive

// Reaches out to huggingface.co so it is not part of the normal test run. Use
//
//	go test -tags hflive -run Live ./slipparquet/
//
// to include it.
package slipparquet

import (
	"fmt"
	"testing"

	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

const imdbURL = "https://huggingface.co/api/datasets/stanfordnlp/imdb/parquet/plain_text/train/0.parquet"

func TestLiveHuggingFace(t *testing.T) {
	r, err := newHTTPReader(imdbURL)
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()
	tt.Equal(t, true, 20_000_000 < r.size)

	pr, err := file.NewParquetReader(r)
	tt.Nil(t, err)
	defer func() { _ = pr.Close() }()

	tt.Equal(t, int64(25000), pr.NumRows())
	tt.Equal(t, 25, pr.NumRowGroups())

	// The feature spec that names the class labels rides along in the file
	// key-value metadata. Step 6 exposes it to SLIP.
	kv := pr.MetaData().KeyValueMetadata()
	tt.NotNil(t, kv.FindValue("huggingface"))
}

// The whole point of steps 1 and 2: browse a real HuggingFace dataset from
// SLIP without downloading it.
func TestLiveHuggingFaceFromSlip(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(fmt.Sprintf(`(setq reader (parquet-open "%s"))`, imdbURL), scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-count)`,
		Expect: "25000",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :column-count)`,
		Expect: "2",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar (lambda (f) (send f :name)) (send (send reader :schema) :fields))`,
		Expect: `("text" "label")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-group-count)`,
		Expect: "25",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "rows" (send reader :row-group-info 0)))`,
		Expect: "1000",
	}).Test(t)
}
