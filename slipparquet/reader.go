// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"strings"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/cl"
	"github.com/ohler55/slip/pkg/flavors"
)

const batchSize int64 = 4096

var readerFlavor *flavors.Flavor

func defReaderFlavor() {
	readerFlavor = flavors.DefFlavor("parquet-reader-flavor",
		map[string]slip.Object{"filepath": nil, "binary-as": nil, "timeout": nil},
		nil,
		slip.List{
			slip.List{
				slip.Symbol(":init-keywords"),
				slip.Symbol(":file"),
				slip.Symbol(":binary-as"),
				slip.Symbol(":timeout"),
			},
			slip.Symbol(":gettable-instance-variables"),
			slip.List{
				slip.Symbol(":documentation"),
				slip.String(`A parquet reader opens a parquet file and can be used to
access the content of that file. The file can be a local file or a remote file
identified by an HTTP or HTTPS URL. The _filepath_ instance variable holds
whichever was used to open the reader, or a list of them when the reader was
opened on several. The _binary-as_ instance variable holds how binary column
values are represented and _timeout_ how long a remote server is given to
answer.`),
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
	var (
		paths             []string
		binaryAs, timeout slip.Object
	)
	for i := 0; i < len(args); i += 2 {
		key, _ := args[i].(slip.Symbol)
		k := string(key)
		switch {
		case strings.EqualFold(":file", k):
			paths = pathsArg(s, args[i+1], ":file", depth)
		case strings.EqualFold(":binary-as", k):
			binaryAs = args[i+1]
		case strings.EqualFold(":timeout", k):
			timeout = args[i+1]
		}
	}
	// An init keyword that matches an instance variable is assigned by the
	// flavor before :init runs and does not appear in the arguments, so pick
	// it up from the instance variable to check it.
	if binaryAs == nil {
		binaryAs = obj.Get("binary-as")
	}
	if timeout == nil {
		timeout = obj.Get("timeout")
	}
	initReader(s, obj, paths, binaryAsArg(s, binaryAs, depth), timeout, depth)

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
parquet file. Remote files are read with range requests so only the parts used are fetched.

A list of files is read as one table which is how a dataset split into shards is opened.`,
			},
			{
				Name: ":binary-as",
				Type: "keyword",
				Text: `How to represent binary column values. Either _:string_ which is
the default or _:octets_.

Parquet stores text and raw bytes in the same physical type and only some writers annotate
which is which. A column that arrives unannotated is a string by default. Pass _:octets_ to get
the raw bytes instead, which is what is wanted for a column holding an image, audio, or any
other binary payload.`,
			},
			{
				Name: ":timeout",
				Type: "fixnum|float",
				Text: `How long to wait, in seconds, for a remote server to answer. The default is 30.

The timeout covers connecting and waiting for the response headers. It deliberately does not
bound the transfer of the body since a single column chunk can legitimately take a long time
to arrive over a slow link. Has no effect on a local file.`,
			},
		},
	}
}

type readerCloseCaller bool

func (caller readerCloseCaller) Call(s *slip.Scope, args slip.List, _ int) slip.Object {
	if _, sh := selfShards(s); sh != nil {
		sh.close()
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
	if _, sh := selfShards(s); sh != nil {
		result = slip.String(sh.first().ParquetReader().MetaData().Version().String())
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
	if _, sh := selfShards(s); sh != nil {
		result = slip.String(sh.first().ParquetReader().MetaData().GetCreatedBy())
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
	if _, sh := selfShards(s); sh != nil {
		result = slip.Fixnum(sh.rowCount())
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
	if _, sh := selfShards(s); sh != nil {
		result = fileMetadata(sh)
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
	if _, sh := selfShards(s); sh != nil {
		result = slip.Fixnum(sh.rowGroupCount())
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
	obj, sh := selfShards(s)
	if len(args) != 1 {
		slip.MethodArgChoicePanic(s, depth, obj, ":row-group-info", len(args), "1")
	}
	if sh != nil {
		result = rowGroupInfo(s, sh, args[0], depth)
	}
	return
}

func (caller readerRowGroupInfoCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":row-group-info",
		Text: `Returns information about one row group as an assoc list.

The keys are _shard_, _rows_, _columns_, _byte-size_, and _compressed-size_. Since this comes
from the file metadata none of the row group data is read.`,
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
	if _, sh := selfShards(s); sh != nil {
		sch, err := sh.first().Schema()
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
	if _, sh := selfShards(s); sh != nil {
		result = makeSchemaInstance(sh.first().ParquetReader().MetaData().Schema.Root())
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
	obj, sh := selfShards(s)
	if sh != nil {
		schem, err := sh.first().Schema()
		if err != nil {
			panic(err)
		}
		columns := make(slip.List, schem.NumFields())
		for i := 0; i < schem.NumFields(); i++ {
			columns[i] = readColumn(sh, i, isOctets(obj))
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
	obj, sh := selfShards(s)
	if len(args) != 1 {
		slip.MethodArgChoicePanic(s, depth, obj, ":column", len(args), "1")
	}
	if sh != nil {
		schem, err := sh.first().Schema()
		if err != nil {
			panic(err)
		}
		if idx := findColIndex(s, schem, args[0], depth); 0 <= idx {
			result = readColumn(sh, idx, isOctets(obj))
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
	obj, sh := selfShards(s)
	if sh != nil {
		ids, offset, limit := parseRowOpts(s, args[1:], depth)
		rows := make(slip.List, 0, rowCap(sh, limit))
		rr := newRowReader(s, sh, ids, args[0], offset, limit, isOctets(obj), depth)
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
	obj, sh := selfShards(s)
	d2 := depth + 1
	fun := cl.ResolveToCaller(s, args[0], d2)
	if sh != nil {
		ids, offset, limit := parseRowOpts(s, args[2:], depth)
		rr := newRowReader(s, sh, ids, args[1], offset, limit, isOctets(obj), depth)
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

// readColumn reads one column across every shard. The values of a dataset
// column are the values of that column in each shard in order.
func readColumn(sh *shards, idx int, octets bool) (result slip.List) {
	for _, fr := range sh.frs {
		cr, _ := fr.GetColumn(sh.context(), idx)
		if cr == nil {
			continue
		}
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
					result = append(result, arrayValue(chunk, i, octets))
				}
			}
		}
	}
	return
}
