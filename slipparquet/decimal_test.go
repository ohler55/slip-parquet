// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"

	"github.com/ohler55/slip-parquet/slipparquet"
)

func dec128(t *testing.T, in string, prec, scale int32) arrow.Array {
	t.Helper()
	b := array.NewDecimal128Builder(memory.DefaultAllocator,
		&arrow.Decimal128Type{Precision: prec, Scale: scale})
	v, err := decimal.Decimal128FromString(in, prec, scale)
	if err != nil {
		t.Fatal(err)
	}
	b.Append(v)
	b.AppendNull()

	return b.NewArray()
}

// A decimal keeps the scale the column declares. A money column of scale 2
// reads back as 10.50 rather than 10.5.
func TestDecimalKeepsScale(t *testing.T) {
	for _, tc := range []struct{ in string }{
		{"10.50"}, {"10.00"}, {"0.00"}, {"-42.10"}, {"1234567.89"},
	} {
		aa := dec128(t, tc.in, 9, 2)
		tt.Equal(t, slip.String(tc.in), slipparquet.ArrayValue(aa, 0), tc.in)
	}
	aa := dec128(t, "0.1000", 9, 4)
	tt.Equal(t, slip.String("0.1000"), slipparquet.ArrayValue(aa, 0))
}

// Every digit survives at the limit of what a decimal128 can hold.
func TestDecimalFullPrecision(t *testing.T) {
	for _, tc := range []struct {
		in          string
		prec, scale int32
	}{
		{"99999999999999999999999999999999999999", 38, 0},
		{"999999999999999999999999999999999999.99", 38, 2},
		{"1234567890123456789012345678.9012345678", 38, 10},
		{"1234567890123456.7891", 20, 4},
	} {
		aa := dec128(t, tc.in, tc.prec, tc.scale)
		tt.Equal(t, slip.String(tc.in), slipparquet.ArrayValue(aa, 0), tc.in)
	}
}

func TestDecimalNull(t *testing.T) {
	aa := dec128(t, "1.23", 9, 2)
	tt.Equal(t, nil, slipparquet.ArrayValue(aa, 1))
}

func TestDecimal256(t *testing.T) {
	b := array.NewDecimal256Builder(memory.DefaultAllocator,
		&arrow.Decimal256Type{Precision: 50, Scale: 2})
	v, err := decimal.Decimal256FromString("123456789012345678901234567890.12", 50, 2)
	if err != nil {
		t.Fatal(err)
	}
	b.Append(v)
	aa := b.NewArray()
	tt.Equal(t, slip.String("123456789012345678901234567890.12"), slipparquet.ArrayValue(aa, 0))
}

func TestDecimal32And64(t *testing.T) {
	b := array.NewDecimal32Builder(memory.DefaultAllocator,
		&arrow.Decimal32Type{Precision: 8, Scale: 3})
	b.Append(decimal.Decimal32(12345))
	tt.Equal(t, slip.String("12.345"), slipparquet.ArrayValue(b.NewArray(), 0))

	b2 := array.NewDecimal64Builder(memory.DefaultAllocator,
		&arrow.Decimal64Type{Precision: 15, Scale: 2})
	b2.Append(decimal.Decimal64(123456789))
	tt.Equal(t, slip.String("1234567.89"), slipparquet.ArrayValue(b2.NewArray(), 0))
}

// byte-array-decimal.parquet is a real file with a scale of 2. Its values used
// to read as "1", "2" and so on because the scale was dropped. They now read
// as "1.00", "2.00" which is what the file says they are.
func TestDecimalFixtureKeepsScale(t *testing.T) {
	scope := reader(t, "byte-array-decimal.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 0 (parquet-column reader "value"))`,
		Expect: `"1.00"`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(nth 23 (parquet-column reader "value"))`,
		Expect: `"24.00"`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "scale" (list (cons "scale" (send (send (send reader :schema) :find "value") :scale)))))`,
		Expect: "2",
	}).Test(t)
}
