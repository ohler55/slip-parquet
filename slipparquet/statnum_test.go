// Copyright (c) 2026, Peter Ohler, All rights reserved.

// statNum is not exported so unlike most of the tests in this package these
// are internal tests. They cover the statistics types that no writer this
// package is tested against produces.
package slipparquet

import (
	"testing"

	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/metadata"
	"github.com/apache/arrow-go/v18/parquet/schema"
	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
)

// statsOf builds an empty stat block for a physical parquet type.
func statsOf(t *testing.T, typ parquet.Type, length int32) metadata.TypedStatistics {
	t.Helper()
	node, err := schema.NewPrimitiveNode("c", parquet.Repetitions.Required, typ, -1, length)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.NewStatistics(schema.NewColumn(node, 0, 0), nil)
}

// An Int96 has no natural lisp form so the encoded bytes are used. Int96 is a
// legacy timestamp that only old writers produce.
func TestStatNumInt96(t *testing.T) {
	st := statsOf(t, parquet.Types.Int96, 12)
	st.(*metadata.Int96Statistics).Update([]parquet.Int96{
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{9, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}, 0)
	tt.Equal(t, true, st.HasMinMax())
	// Both ends come back as the raw encoded bytes rather than nil.
	mn, _ := statNum(st, false).(slip.String)
	mx, _ := statNum(st, true).(slip.String)
	tt.Equal(t, 12, len(string(mn)))
	tt.Equal(t, 12, len(string(mx)))
	tt.NotEqual(t, string(mn), string(mx))
}

// A stat block with nothing in it has neither a min nor a max.
func TestStatNumNoMinMax(t *testing.T) {
	st := statsOf(t, parquet.Types.Int96, 12)
	tt.Equal(t, false, st.HasMinMax())
	tt.Equal(t, nil, statNum(st, false))
	tt.Equal(t, nil, statNum(st, true))
}

// The typed branches are reachable from lisp through fixtures but checking
// them here keeps every physical type in one place.
func TestStatNumTypes(t *testing.T) {
	b := statsOf(t, parquet.Types.Boolean, 0)
	b.(*metadata.BooleanStatistics).Update([]bool{false, true}, 0)
	tt.Equal(t, nil, statNum(b, false))
	tt.Equal(t, slip.True, statNum(b, true))

	i := statsOf(t, parquet.Types.Int32, 0)
	i.(*metadata.Int32Statistics).Update([]int32{-5, 7}, 0)
	tt.Equal(t, slip.Fixnum(-5), statNum(i, false))
	tt.Equal(t, slip.Fixnum(7), statNum(i, true))

	f := statsOf(t, parquet.Types.Float, 0)
	f.(*metadata.Float32Statistics).Update([]float32{1.5, 3.5}, 0)
	tt.Equal(t, slip.SingleFloat(1.5), statNum(f, false))
	tt.Equal(t, slip.SingleFloat(3.5), statNum(f, true))

	d := statsOf(t, parquet.Types.Double, 0)
	d.(*metadata.Float64Statistics).Update([]float64{1.25, 9.75}, 0)
	tt.Equal(t, slip.DoubleFloat(1.25), statNum(d, false))
	tt.Equal(t, slip.DoubleFloat(9.75), statNum(d, true))
}
