// Copyright (c) 2026, Peter Ohler, All rights reserved.

//go:build hflive

// Reaches out to huggingface.co so it is not part of the normal test run. Use
//
//	go test -tags hflive -run Live ./slipparquet/
//
// to include it.
package slipparquet

import (
	"testing"

	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/ohler55/ojg/tt"
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
