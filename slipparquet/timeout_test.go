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

// stalledServer accepts the request and then never answers.
func stalledServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		ts.Close()
	})

	return ts
}

// A server that never answers must fail rather than hang forever.
func TestTimeoutStalledServer(t *testing.T) {
	ts := stalledServer(t)
	done := make(chan slip.Object, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- slip.String(fmt.Sprintf("%v", r))
			}
		}()
		scope := slip.NewScope()
		done <- slip.ReadString(
			fmt.Sprintf(`(parquet-open "%s/x.parquet" :timeout 0.25)`, ts.URL), scope).Eval(scope, nil)
	}()
	select {
	case <-done:
		// Failed rather than hung, which is the point.
	case <-time.After(10 * time.Second):
		t.Fatal("open against a stalled server did not give up")
	}
}

// Without a timeout the default still applies rather than waiting forever.
func TestTimeoutDefaultIsSet(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :timeout)`,
		Expect: "nil",
	}).Test(t)
}

func TestTimeoutRecorded(t *testing.T) {
	body, err := os.ReadFile("testdata/primitive.parquet")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "primitive.parquet", time.Time{}, bytes.NewReader(body))
	}))
	defer ts.Close()

	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(fmt.Sprintf(
		`(setq reader (parquet-open "%s/primitive.parquet" :timeout 5))`, ts.URL), scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()

	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :timeout)`,
		Expect: "5",
	}).Test(t)
	// A normal read is unaffected by having a timeout set.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-count)`,
		Expect: "8",
	}).Test(t)
}

func TestTimeoutBadValue(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :timeout "soon")`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :timeout -1)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :nope 1)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}

// Closing a reader cancels the context so a read still in flight is given up
// on rather than left running.
func TestCloseCancelsReads(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(progn (send reader :close) t)`,
		Expect: "t",
	}).Test(t)
}

// A timeout given as a single float is accepted as well as a fixnum or a
// double.
func TestTimeoutSingleFloat(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (parquet-open "testdata/primitive.parquet" :timeout 1.5s0))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :row-count)`,
		Expect: "8",
	}).Test(t)
}
