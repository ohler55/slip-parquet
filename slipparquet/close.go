// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Close{Function: slip.Function{Name: "parquet-close", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-close",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to close.",
				},
			},
			Return: "nil",
			Text: `__parquet-close__ closes a parquet reader.

This is the same as sending _:close_ to an instance of the _parquet-reader-flavor_ except
that none of the _:close_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-close reader) => nil`,
			},
		}, &Pkg)
}

// Close represents the parquet-close function.
type Close struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Close) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 1)
	sh := readerArg(s, args, depth)
	sh.close()

	return nil
}
