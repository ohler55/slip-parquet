// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"strings"

	"github.com/apache/arrow-go/v18/parquet/metadata"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/apache/arrow-go/v18/parquet/schema"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

const statsDoc = `Statistics are optional in a parquet file. Older writers and some columns
carry none in which case _min_ and _max_ are nil. Statistics are recorded per column chunk so
there is one entry for each row group. The keys are _row-group_, _min_, _max_, _null-count_,
_distinct-count_, and _num-values_.

Statistics are kept for the leaf columns of the file so a nested column is identified by its
dotted path such as _st.a_ rather than by the name of the struct that holds it.

The _min_ and _max_ are the values as physically stored. A column holding a date stores an
integer so the statistics for it are integers.`

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Statistics{Function: slip.Function{Name: "parquet-statistics", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-statistics",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the statistics of.",
				},
				{Name: "&optional"},
				{
					Name: "column-id",
					Type: "string|symbol|fixnum",
					Text: `The path or leaf index of the column. When left out the
statistics for every column are returned as an assoc list keyed by column path.`,
				},
			},
			Return: "list",
			Text: `__parquet-statistics__ returns the column statistics of a parquet reader.

` + statsDoc + `

Since the statistics come from the file metadata none of the column data is read which makes
this a cheap way to see the shape of a file before reading it.

This is the same as sending _:statistics_ to an instance of the _parquet-reader-flavor_ except
that none of the _:statistics_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-statistics reader "text") => ((("row-group" . 0) ("min" . "aaa") ("max" . "zzz") ...))`,
				`(mapcar 'car (parquet-statistics reader)) => ("text" "label")`,
			},
		}, &Pkg)
}

// Statistics represents the parquet-statistics function.
type Statistics struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Statistics) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 2)
	var id slip.Object
	if 1 < len(args) {
		id = args[1]
	}
	return columnStats(s, readerArg(s, args, depth), id, depth)
}

// statNum returns the min or max of a stat block as a lisp object. A column
// with no statistics has neither.
func statNum(st metadata.TypedStatistics, want bool) slip.Object {
	if !st.HasMinMax() {
		return nil
	}
	switch ts := st.(type) {
	case *metadata.BooleanStatistics:
		if pick(ts.Min(), ts.Max(), want) {
			return slip.True
		}
		return nil
	case *metadata.Int32Statistics:
		return slip.Fixnum(pick(ts.Min(), ts.Max(), want))
	case *metadata.Int64Statistics:
		return slip.Fixnum(pick(ts.Min(), ts.Max(), want))
	case *metadata.Float32Statistics:
		return slip.SingleFloat(pick(ts.Min(), ts.Max(), want))
	case *metadata.Float64Statistics:
		return slip.DoubleFloat(pick(ts.Min(), ts.Max(), want))
	case *metadata.ByteArrayStatistics:
		return slip.String(pick(ts.Min(), ts.Max(), want))
	case *metadata.FixedLenByteArrayStatistics:
		return slip.String(pick(ts.Min(), ts.Max(), want))
	}
	// Int96 and Float16 have no natural lisp form so the encoded bytes are used.
	if want {
		return slip.String(st.EncodeMax())
	}
	return slip.String(st.EncodeMin())
}

// pick returns the max when want is true and the min otherwise.
func pick[T any](minVal, maxVal T, want bool) T {
	if want {
		return maxVal
	}
	return minVal
}

// statsFor returns the per row group statistics of one leaf column.
func statsFor(fr *pqarrow.FileReader, col int) slip.Object {
	pr := fr.ParquetReader()
	stats := make(slip.List, 0, pr.NumRowGroups())
	for i := 0; i < pr.NumRowGroups(); i++ {
		cc, err := pr.MetaData().RowGroup(i).ColumnChunk(col)
		if err != nil {
			continue
		}
		entry := slip.List{
			slip.List{slip.String("row-group"), slip.Tail{Value: slip.Fixnum(i)}},
			slip.List{slip.String("num-values"), slip.Tail{Value: slip.Fixnum(cc.NumValues())}},
		}
		st, err := cc.Statistics()
		if err == nil && st != nil {
			var nulls, distinct slip.Object
			if st.HasNullCount() {
				nulls = slip.Fixnum(st.NullCount())
			}
			if st.HasDistinctCount() {
				distinct = slip.Fixnum(st.DistinctCount())
			}
			entry = append(entry,
				slip.List{slip.String("min"), slip.Tail{Value: statNum(st, false)}},
				slip.List{slip.String("max"), slip.Tail{Value: statNum(st, true)}},
				slip.List{slip.String("null-count"), slip.Tail{Value: nulls}},
				slip.List{slip.String("distinct-count"), slip.Tail{Value: distinct}},
			)
		}
		stats = append(stats, entry)
	}
	if len(stats) == 0 {
		return nil
	}
	return stats
}

// columnStats returns the statistics of one column or, when no column is
// named, of every column keyed by column path.
func columnStats(s *slip.Scope, fr *pqarrow.FileReader, id slip.Object, depth int) slip.Object {
	sc := fr.ParquetReader().MetaData().Schema
	if id == nil {
		all := make(slip.List, 0, sc.NumColumns())
		for i := 0; i < sc.NumColumns(); i++ {
			all = append(all, slip.List{
				slip.String(sc.Column(i).Path()),
				slip.Tail{Value: statsFor(fr, i)},
			})
		}
		if len(all) == 0 {
			return nil
		}
		return all
	}
	col := findLeafColumn(s, sc, id, depth)
	if col < 0 {
		slip.ErrorPanic(s, depth, "column %s not found", id)
	}
	return statsFor(fr, col)
}

// findLeafColumn returns the index of a leaf column identified by index, path,
// or name. A negative result means it was not found.
func findLeafColumn(s *slip.Scope, sc *schema.Schema, id slip.Object, depth int) int {
	var name string
	switch tid := id.(type) {
	case slip.Fixnum:
		if 0 <= tid && int(tid) < sc.NumColumns() {
			return int(tid)
		}
		return -1
	case slip.String:
		name = string(tid)
	case slip.Symbol:
		name = string(tid)
	default:
		slip.TypePanic(s, depth, "column-id", id, "fixnum", "string", "symbol")
	}
	// A dotted path is the precise identifier. Fall back to the leaf name so a
	// flat file can be asked for by column name.
	for i := 0; i < sc.NumColumns(); i++ {
		if sc.Column(i).Path() == name {
			return i
		}
	}
	for i := 0; i < sc.NumColumns(); i++ {
		if sc.Column(i).Name() == name {
			return i
		}
	}
	// Spark and other writers use a dotted path with different separators for
	// nested fields so try a case insensitive match as a last resort.
	for i := 0; i < sc.NumColumns(); i++ {
		if strings.EqualFold(sc.Column(i).Path(), name) {
			return i
		}
	}
	return -1
}

type readerStatisticsCaller bool

func (caller readerStatisticsCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if 1 < len(args) {
		slip.MethodArgChoicePanic(s, depth, obj, ":statistics", len(args), "0 or 1")
	}
	var id slip.Object
	if 0 < len(args) {
		id = args[0]
	}
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = columnStats(s, fr, id, depth)
	}
	return
}

func (caller readerStatisticsCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":statistics",
		Text: "Returns the column statistics of the reader file.\n\n" + statsDoc,
		Args: []*slip.DocArg{
			{Name: "&optional"},
			{
				Name: "column-id",
				Type: "string|symbol|fixnum",
				Text: `The path or leaf index of the column. When left out the
statistics for every column are returned as an assoc list keyed by column path.`,
			},
		},
		Return: "list",
	}
}
