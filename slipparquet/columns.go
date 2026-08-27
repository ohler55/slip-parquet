// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Columns{Function: slip.Function{Name: "parquet-columns", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-columns",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the columns of.",
				},
			},
			Return: "list",
			Text: `__parquet-columns__ returns the columns of a parquet reader.

This is the same as sending _:columns_ to an instance of the _parquet-reader-flavor_ except
that none of the _:columns_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-columns reader) => ((1 2 3) ("a" "b" "c"))`,
			},
		}, &Pkg)
}

// Columns represents the parquet-columns function.
type Columns struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Columns) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 1)
	sh, octets := readerOctets(s, args, depth)
	schem, err := sh.first().Schema()
	if err != nil {
		panic(err)
	}
	columns := make(slip.List, schem.NumFields())
	for i := 0; i < schem.NumFields(); i++ {
		columns[i] = readColumn(sh, i, octets)
	}
	return columns
}
