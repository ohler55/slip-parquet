// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := RowGroupInfo{Function: slip.Function{Name: "parquet-row-group-info", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-row-group-info",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the row group information from.",
				},
				{
					Name: "index",
					Type: "fixnum",
					Text: "The index of the row group.",
				},
			},
			Return: "list",
			Text: `__parquet-row-group-info__ returns information about one row group as an assoc list.

The keys are _shard_, _rows_, _columns_, _byte-size_, and _compressed-size_. Since this comes from the
file metadata none of the row group data is read which makes it a cheap way to size up a file
before reading it.

This is the same as sending _:row-group-info_ to an instance of the _parquet-reader-flavor_
except that none of the _:row-group-info_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-row-group-info reader 0) => (("shard" . 0) ("rows" . 8) ("columns" . 11) ...)`,
			},
		}, &Pkg)
}

// RowGroupInfo represents the parquet-row-group-info function.
type RowGroupInfo struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *RowGroupInfo) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 2, 2)

	return rowGroupInfo(s, readerArg(s, args, depth), args[1], depth)
}
