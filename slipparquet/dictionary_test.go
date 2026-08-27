// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"

	"github.com/ohler55/slip-parquet/slipparquet"
)

// dictOf builds a dictionary array of the given index and value types.
func dictOf(t *testing.T, idx, val arrow.DataType, vals []string) arrow.Array {
	t.Helper()
	b := array.NewDictionaryBuilder(memory.DefaultAllocator,
		&arrow.DictionaryType{IndexType: idx, ValueType: val})
	for _, v := range vals {
		if v == "" {
			b.AppendNull()
			continue
		}
		if err := b.AppendValueFromString(v); err != nil {
			t.Fatal(err)
		}
	}
	return b.NewArray()
}

// The typed path has to agree with the JSON path it replaced, for every index
// width and for the value types a dictionary column actually uses.
func TestDictionaryMatchesJSONPath(t *testing.T) {
	for _, idx := range []arrow.DataType{
		arrow.PrimitiveTypes.Int8, arrow.PrimitiveTypes.Uint8,
		arrow.PrimitiveTypes.Int16, arrow.PrimitiveTypes.Uint16,
		arrow.PrimitiveTypes.Int32, arrow.PrimitiveTypes.Uint32,
		arrow.PrimitiveTypes.Int64, arrow.PrimitiveTypes.Uint64,
	} {
		for _, tc := range []struct {
			val  arrow.DataType
			vals []string
		}{
			{arrow.BinaryTypes.String, []string{"neg", "pos", "neg", ""}},
			{arrow.PrimitiveTypes.Int64, []string{"1", "2", "1", ""}},
			{arrow.PrimitiveTypes.Float64, []string{"1.5", "2.5", "1.5", ""}},
		} {
			aa := dictOf(t, idx, tc.val, tc.vals)
			for i := 0; i < aa.Len(); i++ {
				want := wantFromMarshal(aa, i)
				tt.Equal(t, want, slipparquet.ArrayValue(aa, i), idx.String(), " ", tc.val.String(), " ", i)
			}
		}
	}
}

// wantFromMarshal is what the old path produced: the marshal form parsed back.
func wantFromMarshal(aa arrow.Array, i int) slip.Object {
	if aa.IsNull(i) {
		return nil
	}
	d, _ := aa.(*array.Dictionary)

	return slipparquet.JSONToLisp(d.GetOneForMarshal(i))
}

func TestDictionaryValues(t *testing.T) {
	aa := dictOf(t, arrow.PrimitiveTypes.Int32, arrow.BinaryTypes.String,
		[]string{"neg", "pos", "neg", ""})
	tt.Equal(t, slip.String("neg"), slipparquet.ArrayValue(aa, 0))
	tt.Equal(t, slip.String("pos"), slipparquet.ArrayValue(aa, 1))
	// The repeat resolves through the same dictionary entry.
	tt.Equal(t, slip.String("neg"), slipparquet.ArrayValue(aa, 2))
	tt.Equal(t, nil, slipparquet.ArrayValue(aa, 3))
}

// A dictionary of binary values still honours the octets option.
func TestDictionaryOctets(t *testing.T) {
	b := array.NewDictionaryBuilder(memory.DefaultAllocator,
		&arrow.DictionaryType{IndexType: arrow.PrimitiveTypes.Int32, ValueType: arrow.BinaryTypes.Binary})
	bb := b.(*array.BinaryDictionaryBuilder)
	if err := bb.Append([]byte{0x89, 0x50}); err != nil {
		t.Fatal(err)
	}
	aa := b.NewArray()
	tt.Equal(t, slip.String("\x89P"), slipparquet.ArrayValue(aa, 0))
	tt.Equal(t, slip.Octets{0x89, 0x50}, slipparquet.ArrayValueOctets(aa, 0))
}

func dictBench() arrow.Array {
	b := array.NewDictionaryBuilder(memory.DefaultAllocator, &arrow.DictionaryType{
		IndexType: arrow.PrimitiveTypes.Int32, ValueType: arrow.BinaryTypes.String})
	for i := 0; i < 1000; i++ {
		_ = b.AppendValueFromString([]string{"neg", "pos", "neutral"}[i%3])
	}
	return b.NewArray()
}

// BenchmarkDictionaryTyped is the path taken now.
func BenchmarkDictionaryTyped(b *testing.B) {
	aa := dictBench()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < aa.Len(); j++ {
			_ = slipparquet.ArrayValue(aa, j)
		}
	}
}

// BenchmarkDictionaryMarshal is the path taken before, kept for comparison.
func BenchmarkDictionaryMarshal(b *testing.B) {
	aa := dictBench()
	d, _ := aa.(*array.Dictionary)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < aa.Len(); j++ {
			_ = slipparquet.JSONToLisp(d.GetOneForMarshal(j))
		}
	}
}
