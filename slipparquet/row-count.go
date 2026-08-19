// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := RowCount{Function: slip.Function{Name: "parquet-row-count", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-row-count",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the row count of.",
				},
			},
			Return: "nil",
			Text: `__parquet-row-count__ returns the row count of a parquet reader.

This is the same as sending _:row-count_ to an instance of the _parquet-reader-flavor_ except
that none of the _:row-count_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-row-count reader) => 7`,
			},
		}, &Pkg)
}

// RowCount represents the parquet-row-count function.
type RowCount struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *RowCount) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	slip.CheckArgCount(s, depth, f, args, 1, 1)
	inst, ok := args[0].(*flavors.Instance)
	if !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	var fr *pqarrow.FileReader
	if fr, ok = inst.Any.(*pqarrow.FileReader); !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	return slip.Fixnum(fr.ParquetReader().NumRows())
}
