// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"

	"github.com/ohler55/slip-parquet/slipparquet"
)

// octetReader opens a testdata file asking for binary as octets.
func octetReader(t *testing.T, name string) *slip.Scope {
	t.Helper()
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (parquet-open "testdata/`+name+`" :binary-as :octets))`, scope).Eval(scope, nil)
	t.Cleanup(func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) })

	return scope
}

// The default has to stay what it always was. An unannotated byte array in an
// old file is text far more often than it is bytes.
func TestBinaryDefaultIsString(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 0 (parquet-column reader "string_col"))`,
		Expect: `"0"`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (nth 0 (parquet-column reader "date_string_col")))`,
		Expect: "t",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :binary-as)`,
		Expect: "nil",
	}).Test(t)
}

func TestBinaryAsOctets(t *testing.T) {
	scope := octetReader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(send reader :binary-as)`,
		Expect: ":octets",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (nth 0 (parquet-column reader "string_col")))`,
		Expect: "nil",
	}).Test(t)
	// "0" is a single byte of 0x30.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(length (nth 0 (parquet-column reader "string_col")))`,
		Expect: "1",
	}).Test(t)
}

// Asking for :string explicitly is the same as leaving it out.
func TestBinaryAsStringExplicit(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (parquet-open "testdata/primitive.parquet" :binary-as :string))`, scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 0 (parquet-column reader "string_col"))`,
		Expect: `"0"`,
	}).Test(t)
}

// The option has to reach every path that produces values.
func TestBinaryAsOctetsEveryPath(t *testing.T) {
	scope := octetReader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (car (nth 0 (parquet-rows reader :list '("bin")))))`,
		Expect: "nil",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (nth 0 (parquet-column reader "bin")))`,
		Expect: "nil",
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (nth 5 (nth 0 (parquet-columns reader))))`,
		Expect: "nil",
	}).Test(t)
	// A null binary is still nil rather than empty octets.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(car (nth 1 (parquet-rows reader :list '("bin"))))`,
		Expect: "nil",
	}).Test(t)
	// Text columns are unaffected, only binary ones change.
	(&sliptest.Function{
		Scope:  scope,
		Source: `(car (nth 0 (parquet-rows reader :list '("str"))))`,
		Expect: `"a"`,
	}).Test(t)
}

func TestBinaryAsBadValue(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :binary-as :bytes)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :bogus :octets)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-open "testdata/primitive.parquet" :binary-as)`,
		PanicType: slip.Symbol("error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(make-instance 'parquet-reader-flavor :file "testdata/primitive.parquet" :binary-as :nope)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}

func TestMakeInstanceBinaryAs(t *testing.T) {
	scope := slip.NewScope()
	scope.Let("reader", nil)
	_ = slip.ReadString(
		`(setq reader (make-instance 'parquet-reader-flavor :file "testdata/primitive.parquet" :binary-as :octets))`,
		scope).Eval(scope, nil)
	defer func() { _ = slip.ReadString(`(send reader :close)`, scope).Eval(scope, nil) }()
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (nth 0 (parquet-column reader "string_col")))`,
		Expect: "nil",
	}).Test(t)
}

// The exported converters keep their old behaviour and gain an octets variant.
func TestArrayValueOctets(t *testing.T) {
	b := array.NewBinaryBuilder(memory.DefaultAllocator, arrow.BinaryTypes.Binary)
	b.Append([]byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff}) // PNG magic with a NUL
	b.AppendNull()
	aa := b.NewBinaryArray()

	tt.Equal(t, slip.String("\x89PNG\x00\xff"), slipparquet.ArrayValue(aa, 0))
	tt.Equal(t, slip.Octets{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff}, slipparquet.ArrayValueOctets(aa, 0))
	tt.Equal(t, nil, slipparquet.ArrayValue(aa, 1))
	tt.Equal(t, nil, slipparquet.ArrayValueOctets(aa, 1))
}

// A HuggingFace image feature is a struct of bytes and path so the bytes reach
// lisp through the JSON path rather than the typed one.
func TestJSONToLispOctets(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4e, 0x47}
	tt.Equal(t, slip.String("\x89PNG"), slipparquet.JSONToLisp(raw))
	tt.Equal(t, slip.Octets{0x89, 0x50, 0x4e, 0x47}, slipparquet.JSONToLispOctets(raw))

	nested := map[string]any{"bytes": raw}
	tt.Equal(t, slip.List{slip.List{slip.String("bytes"), slip.Tail{Value: slip.String("\x89PNG")}}},
		slipparquet.JSONToLisp(nested))
	tt.Equal(t, slip.List{slip.List{slip.String("bytes"), slip.Tail{Value: slip.Octets{0x89, 0x50, 0x4e, 0x47}}}},
		slipparquet.JSONToLispOctets(nested))
}

// The nested struct case is what a HuggingFace image column looks like.
func TestBinaryAsOctetsNested(t *testing.T) {
	scope := octetReader(t, "nulls.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "a" (car (nth 0 (parquet-rows reader :list '("st"))))))`,
		Expect: "5",
	}).Test(t)
}
