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

// multi-group.parquet holds 15 rows of id and name in 3 row groups of 5 which
// is what makes row group skipping observable.
func TestPagingFixtureShape(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
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
}

func TestRowsLimit(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :limit 3)`,
		Expect: `((0) (1) (2))`,
	}).Test(t)
	// A limit past the end just gives what is there.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(length (parquet-rows reader :list '("id") :limit 100))`,
		Expect: "15",
	}).Test(t)
	// A zero limit gives nothing.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :limit 0)`,
		Expect: "nil",
	}).Test(t)
}

func TestRowsOffset(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	// An offset inside the first row group.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 2 :limit 3)`,
		Expect: `((2) (3) (4))`,
	}).Test(t)
	// An offset landing exactly on the second row group.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 5 :limit 3)`,
		Expect: `((5) (6) (7))`,
	}).Test(t)
	// An offset that has to skip a whole group then part of the next.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 7 :limit 4)`,
		Expect: `((7) (8) (9) (10))`,
	}).Test(t)
	// An offset past the end gives nothing.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 100)`,
		Expect: "nil",
	}).Test(t)
	// An offset with no limit runs to the end.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :offset 12)`,
		Expect: `((12) (13) (14))`,
	}).Test(t)
}

// Every offset and limit combination has to match the same slice taken from a
// full read. This is the real check on the row group arithmetic.
func TestRowsPagingMatchesFullRead(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope: scope,
		Source: `(let ((all (parquet-rows reader :list '("id" "name")))
                     (bad nil))
                   (dotimes (off 17)
                     (dotimes (lim 17)
                       (let ((want (subseq all (min off 15) (min (+ off lim) 15)))
                             (got (parquet-rows reader :list '("id" "name") :offset off :limit lim)))
                         (unless (equal want got)
                           (setq bad (add bad (list off lim want got)))))))
                   bad)`,
		Expect: "nil",
	}).Test(t)
}

// Paging has to work the same when the columns are defaulted.
func TestRowsPagingAllColumns(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list nil :offset 6 :limit 2)`,
		Expect: `((6 "row-06") (7 "row-07"))`,
	}).Test(t)
	// Leaving the column-ids out entirely still allows keywords.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list :offset 6 :limit 2)`,
		Expect: `((6 "row-06") (7 "row-07"))`,
	}).Test(t)
}

func TestRowsPagingAssoc(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :assoc '("id") :offset 9 :limit 2)`,
		Expect: `((("id" . 9)) (("id" . 10)))`,
	}).Test(t)
}

func TestRowsMethodPaging(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :rows :list '("id") :offset 5 :limit 2)`,
		Expect: `((5) (6))`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :rows :list :limit 2)`,
		Expect: `((0 "row-00") (1 "row-01"))`,
	}).Test(t)
}

func TestEachRowPaging(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	scope.Let("seen", nil)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (parquet-each-row reader (lambda (row) (setq seen (add seen (car row))))
                                     :list '("id") :offset 4 :limit 3)
                   seen)`,
		Expect: `(4 5 6)`,
	}).Test(t)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (send reader :each-row (lambda (row) (setq seen (add seen (car row))))
                         :list :limit 2)
                   seen)`,
		Expect: `(0 1)`,
	}).Test(t)
}

// A non local exit out of the callback stops the walk. The reader belongs to
// the caller so nothing is leaked by leaving early.
func TestEachRowEarlyExit(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	scope.Let("seen", nil)
	(&sliptest.Function{
		Scope: scope,
		Source: `(progn
                   (setq seen nil)
                   (block walk
                     (parquet-each-row reader
                                       (lambda (row)
                                         (when (< 3 (car row)) (return-from walk))
                                         (setq seen (add seen (car row))))
                                       :list '("id")))
                   seen)`,
		Expect: `(0 1 2 3)`,
	}).Test(t)
	// The reader is still usable after the early exit.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-rows reader :list '("id") :limit 2)`,
		Expect: `((0) (1))`,
	}).Test(t)
}

func TestPagingBadArgs(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-rows reader :list '("id") :limit "two")`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-rows reader :list '("id") :offset -1)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-rows reader :list '("id") :bogus 1)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Scope:     scope,
		Source:    `(parquet-rows reader :list '("id") :limit)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
}

// Paging must not disturb the existing behaviour of a plain read.
func TestRowsUnpagedUnchanged(t *testing.T) {
	scope := reader(t, "multi-group.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(length (parquet-rows reader :list))`,
		Expect: "15",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (parquet-rows reader :list) (parquet-rows reader :list :offset 0))`,
		Expect: "t",
	}).Test(t)
}

// A limited read of a remote file has to actually transfer less than a full
// read, otherwise the row group selection is not doing its job.
func TestPagingRemoteFetchesLess(t *testing.T) {
	body, err := os.ReadFile("testdata/multi-group.parquet")
	if err != nil {
		t.Fatal(err)
	}
	var served int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		http.ServeContent(rec, r, "multi-group.parquet", time.Time{}, bytes.NewReader(body))
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		b := rec.Body.Bytes()
		served += int64(len(b))
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	read := func(src string) int64 {
		scope := slip.NewScope()
		scope.Let("reader", nil)
		_ = slip.ReadString(
			fmt.Sprintf(`(setq reader (parquet-open "%s/multi-group.parquet"))`, ts.URL), scope).Eval(scope, nil)
		served = 0
		_ = slip.ReadString(src, scope).Eval(scope, nil)
		n := served
		_ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil)

		return n
	}
	full := read(`(parquet-rows reader :list)`)
	limited := read(`(parquet-rows reader :list :limit 1)`)
	if limited >= full {
		t.Fatalf("limited read fetched %d bytes, full read fetched %d, expected less", limited, full)
	}
	t.Logf("full read %d bytes, :limit 1 read %d bytes", full, limited)
}
