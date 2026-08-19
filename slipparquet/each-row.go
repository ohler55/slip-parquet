// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/cl"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := EachRow{Function: slip.Function{Name: "parquet-each-row", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-each-row",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the each-row of.",
				},
				{
					Name: "function",
					Type: "function",
					Text: "The funcion to call for each of the reader.",
				},
				{
					Name: "format",
					Type: "keyword",
					Text: `The format of each row. Valid values are _:list_ for a list of
column values in order or _:assoc_ for column names as the car and the values as the cdr
of each element in an assoc list for each key value pair.`,
				},
				{Name: "&optional"},
				{
					Name: "column-ids",
					Type: "list",
					Text: `A list of the column indexes or names for the columns to get values
from. The default is all columns`,
				},
				{Name: "&key"},
				{
					Name: ":offset",
					Type: "fixnum",
					Text: "The number of rows to skip. The default is 0.",
				},
				{
					Name: ":limit",
					Type: "fixnum",
					Text: `The maximum number of rows. The default is all of them.

Only the row groups holding the requested rows are read so paging through a large
or remote file does not read the whole file.`,
				},
			},
			Return: "nil",
			Text: `__parquet-each-row__ calls the _function_ for each row in the parquet
reader in the _format_ specified and with the columns identified by the _column-ids_.

To stop before the last row use a _block_ and _return-from_ in the _function_. The reader
is not closed by leaving early so it can still be used afterwards.

This is the same as sending _:each-row_ to an instance of the _parquet-reader-flavor_ except
that none of the _:each-row_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet") rows nil)`,
				`(parquet-each-row reader (lambda (row) (setq rows (add rows row))) :assoc) => nil`,
				`  ;; rows will contain an assoc list of all rows`,
			},
		}, &Pkg)
}

// EachRow represents the parquet-each-row function.
type EachRow struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *EachRow) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 3, 8)
	fr := readerArg(s, args, depth)
	d2 := depth + 1
	fun := cl.ResolveToCaller(s, args[1], d2)
	ids, offset, limit := parseRowOpts(s, args[3:], depth)
	rr := newRowReader(s, fr, ids, args[2], offset, limit, depth)
	for {
		row := rr.next()
		if row == nil {
			break
		}
		_ = fun.Call(s, slip.List{row}, d2)
	}
	return nil
}
