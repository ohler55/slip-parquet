// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Metadata{Function: slip.Function{Name: "parquet-metadata", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-metadata",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the metadata of.",
				},
			},
			Return: "list",
			Text: `__parquet-metadata__ returns the file key-value metadata as an assoc list.

Every parquet file can carry a map of strings in its footer. What is in it depends on the
application that wrote the file. Spark writes its SQL schema under
_org.apache.spark.sql.parquet.row.metadata_, Arrow writes a serialized schema under
_ARROW:schema_, and a HuggingFace dataset carries the feature spec under _huggingface_.
The values are returned as written so a value holding JSON has to be parsed by the caller.

Returns nil when the file has no key-value metadata.

This is the same as sending _:metadata_ to an instance of the _parquet-reader-flavor_ except
that none of the _:metadata_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(mapcar 'car (parquet-metadata reader)) => ("huggingface" "ARROW:schema")`,
				`(cdr (assoc "huggingface" (parquet-metadata reader))) => "{\"info\": ...}"`,
			},
		}, &Pkg)
}

// Metadata represents the parquet-metadata function.
type Metadata struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Metadata) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 1)

	return fileMetadata(readerArg(s, args, depth))
}
