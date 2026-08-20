// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := RowGroupCount{Function: slip.Function{Name: "parquet-row-group-count", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-row-group-count",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the row group count of.",
				},
			},
			Return: "fixnum",
			Text: `__parquet-row-group-count__ returns the number of row groups in a parquet reader.

A parquet file is stored as one or more row groups. Row groups are the unit the file is read
in so they are a convenient way to step through a large file a chunk at a time.

This is the same as sending _:row-group-count_ to an instance of the _parquet-reader-flavor_
except that none of the _:row-group-count_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-row-group-count reader) => 1`,
			},
		}, &Pkg)
}

// RowGroupCount represents the parquet-row-group-count function.
type RowGroupCount struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *RowGroupCount) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 1)

	return slip.Fixnum(readerArg(s, args, depth).rowGroupCount())
}
