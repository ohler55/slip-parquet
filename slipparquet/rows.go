// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Rows{Function: slip.Function{Name: "parquet-rows", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-rows",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the rows of.",
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
			Return: "list",
			Text: `__parquet-rows__ returns the rows of a parquet reader in the _format_
specified and with the columns identified by the _column-ids_.

This is the same as sending _:rows_ to an instance of the _parquet-reader-flavor_ except
that none of the _:rows_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-rows reader :assoc '(0 2)) => ((("a" . 1)("c" . t)) (("a" . 2) ("c" . nil)))`,
				`(parquet-rows reader :list nil :offset 100 :limit 10) ;; rows 100 through 109`,
				`(parquet-rows reader :list :limit 10) ;; the first 10 rows of all columns`,
			},
		}, &Pkg)
}

// Rows represents the parquet-rows function.
type Rows struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Rows) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 2, 7)
	fr, octets := readerOctets(s, args, depth)
	ids, offset, limit := parseRowOpts(s, args[2:], depth)
	rows := make(slip.List, 0, rowCap(fr, limit))
	rr := newRowReader(s, fr, ids, args[1], offset, limit, octets, depth)
	for {
		row := rr.next()
		if row == nil {
			break
		}
		rows = append(rows, row)
	}
	return rows
}
