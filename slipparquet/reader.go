// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"context"
	"strings"

	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/cl"
	"github.com/ohler55/slip/pkg/flavors"
)

const batchSize int64 = 4096

var readerFlavor *flavors.Flavor

func defReaderFlavor() {
	readerFlavor = flavors.DefFlavor("parquet-reader-flavor",
		map[string]slip.Object{"filepath": nil},
		nil,
		slip.List{
			slip.List{
				slip.Symbol(":init-keywords"),
				slip.Symbol(":file"),
			},
			slip.Symbol(":gettable-instance-variables"),
			slip.List{
				slip.Symbol(":documentation"),
				slip.String(`A parquet reader opens a parquet file and can be used to
access the content of that file.`),
			},
		},
		&Pkg,
	)
	readerFlavor.DefMethod(":init", "", readerInitCaller(true))
	readerFlavor.DefMethod(":close", "", readerCloseCaller(true))
	readerFlavor.DefMethod(":version", "", readerVersionCaller(true))
	readerFlavor.DefMethod(":created-by", "", readerCreatedByCaller(true))
	readerFlavor.DefMethod(":schema", "", readerSchemaCaller(true))

	readerFlavor.DefMethod(":column-count", "", readerColumnCountCaller(true))
	readerFlavor.DefMethod(":columns", "", readerColumnsCaller(true))
	readerFlavor.DefMethod(":column", "", readerColumnCaller(true))

	readerFlavor.DefMethod(":row-count", "", readerRowCountCaller(true))
	readerFlavor.DefMethod(":rows", "", readerRowsCaller(true))
	readerFlavor.DefMethod(":each-row", "", readerEachRowCaller(true))
}

type readerInitCaller bool

func (caller readerInitCaller) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	obj := s.Get("self").(*flavors.Instance)
	if 0 < len(args) {
		args = args[0].(slip.List)
	}
	var path string
	for i := 0; i < len(args); i += 2 {
		key, _ := args[i].(slip.Symbol)
		k := string(key)
		if strings.EqualFold(":file", k) {
			ss, ok := args[i+1].(slip.String)
			if !ok {
				slip.TypePanic(s, depth, ":file", args[i+1], "string")
			}
			path = string(ss)
		}
	}
	initReader(obj, path)

	return nil
}

func (caller readerInitCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":init",
		Text: "Sets the initial value when _make-instance_ is called.",
		Args: []*slip.DocArg{
			{Name: "&key"},
			{
				Name: ":file",
				Type: "string",
				Text: "The path to a parquet file.",
			},
		},
	}
}

type readerCloseCaller bool

func (caller readerCloseCaller) Call(s *slip.Scope, args slip.List, _ int) slip.Object {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		_ = fr.ParquetReader().Close()
	}
	return nil
}

func (caller readerCloseCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":close",
		Text: "Close the reader.",
	}
}

type readerVersionCaller bool

func (caller readerVersionCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = slip.String(fr.ParquetReader().MetaData().Version().String())
	}
	return
}

func (caller readerVersionCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":version",
		Text:   "Returns the version of the reader file.",
		Return: "string",
	}
}

type readerCreatedByCaller bool

func (caller readerCreatedByCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = slip.String(fr.ParquetReader().MetaData().GetCreatedBy())
	}
	return
}

func (caller readerCreatedByCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":created-by",
		Text:   "Returns a description of the application that created of the reader file.",
		Return: "string",
	}
}

type readerRowCountCaller bool

func (caller readerRowCountCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = slip.Fixnum(fr.ParquetReader().NumRows())
	}
	return
}

func (caller readerRowCountCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":row-count",
		Text:   "Returns the number of rows in the reader file.",
		Return: "fixnum",
	}
}

type readerColumnCountCaller bool

func (caller readerColumnCountCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		sch, err := fr.Schema()
		if err != nil {
			panic(err)
		}
		result = slip.Fixnum(sch.NumFields())
	}
	return
}

func (caller readerColumnCountCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":column-count",
		Text:   "Returns the number of columns in the reader file.",
		Return: "fixnum",
	}
}

type readerSchemaCaller bool

func (caller readerSchemaCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = makeSchemaInstance(fr.ParquetReader().MetaData().Schema.Root())
	}
	return
}

func (caller readerSchemaCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":schema",
		Text:   "Returns the schema of the reader file.",
		Return: "list",
	}
}

type readerColumnsCaller bool

func (caller readerColumnsCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		schem, err := fr.Schema()
		if err != nil {
			panic(err)
		}
		columns := make(slip.List, schem.NumFields())
		for i := 0; i < schem.NumFields(); i++ {
			if cr, _ := fr.GetColumn(context.Background(), i); cr != nil {
				columns[i] = readColumn(cr)
			}
		}
		result = columns
	}
	return
}

func (caller readerColumnsCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":columns",
		Text:   "Returns the columns of the reader file.",
		Return: "list",
	}
}

type readerColumnCaller bool

func (caller readerColumnCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if len(args) != 1 {
		slip.MethodArgChoicePanic(s, depth, obj, ":column", len(args), "1")
	}
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		schem, err := fr.Schema()
		if err != nil {
			panic(err)
		}
		if cr, _ := findColReader(s, fr, schem, args[0], depth); cr != nil {
			result = readColumn(cr)
		}
	}
	return
}

func (caller readerColumnCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":column",
		Text: "Returns the specified column of the reader file.",
		Args: []*slip.DocArg{
			{
				Name: "id",
				Type: "string|fixnum",
				Text: "The name or numeric index of the column to read.",
			},
		},
		Return: "list",
	}
}

func (caller readerRowsCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		rowCnt := fr.ParquetReader().NumRows()
		rows := make(slip.List, 0, rowCnt)
		var ids slip.Object
		if 1 < len(args) {
			ids = args[1]
		}
		rr := newRowReader(s, fr, ids, args[0], depth)
		for {
			row := rr.next()
			if row == nil {
				break
			}
			rows = append(rows, row)
		}
		result = rows
	}
	return
}

func (caller readerRowsCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":rows",
		Text: "Returns the rows of the reader file.",
		Args: []*slip.DocArg{
			{
				Name: "format",
				Type: "symbol",
				Text: `Can be one of _:list_ for a list of column values in order,
_:assoc_ for column names as the car and the values as the cdr of each
element in an assoc list for each key value pair.`,
			},
			{Name: "&optional"},
			{
				Name: "column-ids",
				Type: "list",
				Text: "A list of the column indexes or names for the columns to get values from.",
			},
		},
		Return: "list",
	}
}

type readerEachRowCaller bool

func (caller readerEachRowCaller) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	obj := s.Get("self").(*flavors.Instance)
	d2 := depth + 1
	fun := cl.ResolveToCaller(s, args[0], d2)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		var ids slip.Object
		if 2 < len(args) {
			ids = args[2]
		}
		rr := newRowReader(s, fr, ids, args[1], depth)
		for {
			row := rr.next()
			if row == nil {
				break
			}
			_ = fun.Call(s, slip.List{row}, d2)
		}
	}
	return nil
}

func (caller readerEachRowCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":each-row",
		Text: "Calls the _function_ on each row.",
		Args: []*slip.DocArg{
			{
				Name: "function",
				Type: "symbol|lambda",
				Text: "A function or lambda to call on each row.",
			},
			{
				Name: "format",
				Type: "symbol",
				Text: `Can be one of _:list_ for a list of column values in order,
_:assoc_ for column names as the car and the values as the cdr of each
element in an assoc list for each key value pair.`,
			},
			{Name: "&optional"},
			{
				Name: "column-ids",
				Type: "list",
				Text: "A list of the column indexes or names for the columns to get values from.",
			},
		},
		Return: "list",
	}
}

func readColumn(cr *pqarrow.ColumnReader) (result slip.List) {
	for {
		ac, err := cr.NextBatch(batchSize) // *arrow.Chunked
		if err != nil {
			panic(err)
		}
		if ac.Len() == 0 {
			break
		}
		for _, chunk := range ac.Chunks() {
			cnt := chunk.Len()
			for i := 0; i < cnt; i++ {
				result = append(result, ArrayValue(chunk, i))
			}
		}
	}
	return
}
