// Copyright (c) 2023, Peter Ohler, All rights reserved.

package main_test

import (
	"testing"
	"time"

	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	sp "github.com/ohler55/slip-parquet"
)

func TestJSONToLisp(t *testing.T) {
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(slip.Fixnum(3)))

	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(3))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(int8(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(int16(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(int32(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(int64(3)))

	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(uint(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(uint8(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(uint16(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(uint32(3)))
	tt.Equal(t, slip.Fixnum(3), sp.JSONToLisp(uint64(3)))

	tt.Equal(t, slip.SingleFloat(2.5), sp.JSONToLisp(float32(2.5)))
	tt.Equal(t, slip.DoubleFloat(2.5), sp.JSONToLisp(float64(2.5)))

	tt.Equal(t, slip.String("abc"), sp.JSONToLisp("abc"))
	tt.Equal(t, slip.String("abc"), sp.JSONToLisp([]byte("abc")))

	tm := time.Now()
	tt.Equal(t, slip.Time(tm), sp.JSONToLisp(tm))
}
