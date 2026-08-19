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
access the content of that file. The file can be a local file or a remote file
identified by an HTTP or HTTPS URL. The _filepath_ instance variable holds
whichever was used to open the reader.`),
			},
		},
		&Pkg,
	)
	readerFlavor.DefMethod(":init", "", readerInitCaller(true))
	readerFlavor.DefMethod(":close", "", readerCloseCaller(true))
	readerFlavor.DefMethod(":version", "", readerVersionCaller(true))
	readerFlavor.DefMethod(":created-by", "", readerCreatedByCaller(true))
	readerFlavor.DefMethod(":schema", "", readerSchemaCaller(true))
	readerFlavor.DefMethod(":metadata", "", readerMetadataCaller(true))
	readerFlavor.DefMethod(":statistics", "", readerStatisticsCaller(true))

	readerFlavor.DefMethod(":column-count", "", readerColumnCountCaller(true))
	readerFlavor.DefMethod(":columns", "", readerColumnsCaller(true))
	readerFlavor.DefMethod(":column", "", readerColumnCaller(true))

	readerFlavor.DefMethod(":row-group-count", "", readerRowGroupCountCaller(true))
	readerFlavor.DefMethod(":row-group-info", "", readerRowGroupInfoCaller(true))

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
	initReader(s, obj, path, depth)

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
				Text: `The path to a parquet file or an HTTP or HTTPS URL of a remote
parquet file. Remote files are read with range requests so only the parts used are fetched.`,
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

type readerMetadataCaller bool

func (caller readerMetadataCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = fileMetadata(fr)
	}
	return
}

func (caller readerMetadataCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":metadata",
		Text: `Returns the file key-value metadata as an assoc list.

Every parquet file can carry a map of strings in its footer. What is in it depends on the
application that wrote the file. Spark writes its SQL schema under
_org.apache.spark.sql.parquet.row.metadata_, Arrow writes a serialized schema under
_ARROW:schema_, and a HuggingFace dataset carries the feature spec under _huggingface_.
The values are returned as written so a value holding JSON has to be parsed by the caller.

Returns nil when the file has no key-value metadata.`,
		Return: "list",
	}
}

type readerRowGroupCountCaller bool

func (caller readerRowGroupCountCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = slip.Fixnum(fr.ParquetReader().NumRowGroups())
	}
	return
}

func (caller readerRowGroupCountCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":row-group-count",
		Text: `Returns the number of row groups in the reader file.

A parquet file is stored as one or more row groups. Row groups are the unit the file is read
in so they are a convenient way to step through a large file a chunk at a time.`,
		Return: "fixnum",
	}
}

type readerRowGroupInfoCaller bool

func (caller readerRowGroupInfoCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if len(args) != 1 {
		slip.MethodArgChoicePanic(s, depth, obj, ":row-group-info", len(args), "1")
	}
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		result = rowGroupInfo(s, fr, args[0], depth)
	}
	return
}

func (caller readerRowGroupInfoCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":row-group-info",
		Text: `Returns information about one row group as an assoc list.

The keys are _rows_, _columns_, _byte-size_, and _compressed-size_. Since this comes from the
file metadata none of the row group data is read.`,
		Args: []*slip.DocArg{
			{
				Name: "index",
				Type: "fixnum",
				Text: "The index of the row group.",
			},
		},
		Return: "list",
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
		if cr, _ := findColReader(s, fr, schem, args[0], nil, depth); cr != nil {
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
		ids, offset, limit := parseRowOpts(s, args[1:], depth)
		rows := make(slip.List, 0, rowCap(fr, limit))
		rr := newRowReader(s, fr, ids, args[0], offset, limit, depth)
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
	}
}

type readerEachRowCaller bool

func (caller readerEachRowCaller) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	obj := s.Get("self").(*flavors.Instance)
	d2 := depth + 1
	fun := cl.ResolveToCaller(s, args[0], d2)
	if fr, ok := obj.Any.(*pqarrow.FileReader); ok {
		ids, offset, limit := parseRowOpts(s, args[2:], depth)
		rr := newRowReader(s, fr, ids, args[1], offset, limit, depth)
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
		Text: `Calls the _function_ on each row.

To stop before the last row use a _block_ and _return-from_ in the _function_. The reader
is not closed by leaving early so it can still be used afterwards.`,
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
