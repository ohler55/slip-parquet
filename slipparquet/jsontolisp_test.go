// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"
	"time"

	"github.com/ohler55/ojg/tt"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip-parquet/slipparquet"
)

func TestJSONToLisp(t *testing.T) {
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(slip.Fixnum(3)))

	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(3))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(int8(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(int16(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(int32(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(int64(3)))

	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(uint(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(uint8(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(uint16(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(uint32(3)))
	tt.Equal(t, slip.Fixnum(3), slipparquet.JSONToLisp(uint64(3)))

	tt.Equal(t, slip.SingleFloat(2.5), slipparquet.JSONToLisp(float32(2.5)))
	tt.Equal(t, slip.DoubleFloat(2.5), slipparquet.JSONToLisp(float64(2.5)))

	tt.Equal(t, slip.String("abc"), slipparquet.JSONToLisp("abc"))
	tt.Equal(t, slip.String("abc"), slipparquet.JSONToLisp([]byte("abc")))

	tm := time.Now()
	tt.Equal(t, slip.Time(tm), slipparquet.JSONToLisp(tm))
}
