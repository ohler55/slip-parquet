// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// parquetServer serves a testdata file over HTTP with range request support,
// which is what a remote parquet reader needs.
func parquetServer(t *testing.T, name string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(ts.Close)

	return ts
}

func TestOpenURL(t *testing.T) {
	ts := parquetServer(t, "primitive.parquet")
	scope := slip.NewScope()
	scope.Let("reader", nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: fmt.Sprintf(`(setq reader (parquet-open "%s/primitive.parquet"))`, ts.URL),
		Expect: "/#<parquet-reader-flavor [0-9a-f]+>/",
	}).Test(t)
}

// A remote reader has to answer everything a local one does.
func TestOpenURLSameAsLocal(t *testing.T) {
	ts := parquetServer(t, "primitive.parquet")
	for _, src := range []string{
		`(send reader :row-count)`,
		`(send reader :column-count)`,
		`(send reader :created-by)`,
		`(send reader :version)`,
		`(send reader :column "id")`,
		`(send reader :rows :list '("id" "bool_col"))`,
	} {
		local := slip.NewScope()
		local.Let("reader", nil)
		_ = slip.ReadString(`(setq reader (parquet-open "testdata/primitive.parquet"))`, local).Eval(local, nil)
		want := slip.ReadString(src, local).Eval(local, nil)
		_ = slip.ReadString(`(send reader :close)`, local).Eval(local, nil)

		remote := slip.NewScope()
		remote.Let("reader", nil)
		_ = slip.ReadString(
			fmt.Sprintf(`(setq reader (parquet-open "%s/primitive.parquet"))`, ts.URL), remote).Eval(remote, nil)
		defer func() { _ = slip.ReadString(`(send reader :close)`, remote).Eval(remote, nil) }()

		(&sliptest.Function{
			Scope:  remote,
			Source: src,
			Expect: string(slip.ObjectAppend(nil, want)),
		}).Test(t)
	}
}

func TestOpenURLFilepath(t *testing.T) {
	ts := parquetServer(t, "primitive.parquet")
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		fmt.Sprintf(`(setq reader (parquet-open "%s/primitive.parquet"))`, ts.URL), scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :filepath)`,
		Expect: `"` + ts.URL + `/primitive.parquet"`,
	}).Test(t)
}

// make-instance with a :file of a URL has to work the same as parquet-open.
func TestMakeInstanceURL(t *testing.T) {
	ts := parquetServer(t, "primitive.parquet")
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		fmt.Sprintf(`(setq reader (make-instance 'parquet-reader-flavor :file "%s/primitive.parquet"))`,
			ts.URL), scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-count)`,
		Expect: "8",
	}).Test(t)
}

// Closing a remote reader releases the HTTP reader under it. Closing twice
// must not blow up either.
func TestCloseURL(t *testing.T) {
	ts := parquetServer(t, "primitive.parquet")
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		fmt.Sprintf(`(setq reader (parquet-open "%s/primitive.parquet"))`, ts.URL), scope).Eval(scope, nil)

	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :close)`,
		Expect: "nil",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-close reader)`,
		Expect: "nil",
	}).Test(t)
}

func TestOpenURLNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer ts.Close()

	(&sliptest.Function{
		Source:    fmt.Sprintf(`(parquet-open "%s/missing.parquet")`, ts.URL),
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}

func TestOpenURLNoRangeSupport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "8")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	(&sliptest.Function{
		Source:    fmt.Sprintf(`(parquet-open "%s/sample.parquet")`, ts.URL),
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}

func TestOpenURLUnreachable(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open "http://127.0.0.1:0/nothing.parquet")`,
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}

// Serving something that is not parquet has to fail as a file error rather
// than escaping as a raw Go panic.
func TestOpenURLNotParquet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "sample.parquet", time.Time{}, bytes.NewReader([]byte("not a parquet file")))
	}))
	defer ts.Close()

	(&sliptest.Function{
		Source:    fmt.Sprintf(`(parquet-open "%s/sample.parquet")`, ts.URL),
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}

// A missing local file now raises a file-error too rather than a raw panic.
func TestOpenMissingLocalFile(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/does-not-exist.parquet")`,
		PanicType: slip.Symbol("file-error"),
	}).Test(t)
}
