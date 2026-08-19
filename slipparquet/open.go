// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"strings"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Open{Function: slip.Function{Name: "parquet-open", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-open",
			Args: []*slip.DocArg{
				{
					Name: "file",
					Type: "string",
					Text: "The parquet file to read. Either a local path or an HTTP or HTTPS URL.",
				},
				{Name: "&key"},
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
			},
			Return: "instance",
			Text: `__parquet-open__ opens a parquet file and returns a _parquet-reader-flavor_ instance.

If the _file_ begins with _http://_ or _https://_ the file is read remotely using range
requests. Only the parts of the file actually used are fetched so a large remote file can be
examined without downloading all of it. The server must support range requests.

When reading from _huggingface.co_ the _HF_TOKEN_ environment variable, if set, is used to
authorize the request. The token is never sent to any other host.

This is the same as the calling _make-instance_ of the _parquet-reader-flavor_ with the _:file_ init
option of _:file_ except that none of the _:init_ daemons are called.`,
			Examples: []string{
				`(parquet-open "sample.parquet") => #<parquet-reader-flavor 1234>`,
				`(parquet-open "https://huggingface.co/api/datasets/stanfordnlp/imdb/parquet/plain_text/train/0.parquet")`,
				`(parquet-open "images.parquet" :binary-as :octets)`,
			},
		}, &Pkg)
}

// Open represents the parquet-open function.
type Open struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Open) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 3)
	path, ok := args[0].(slip.String)
	if !ok {
		slip.TypePanic(s, depth, "file", args[0], "string")
	}
	var binaryAs slip.Object
	for i := 1; i < len(args); i += 2 {
		sym, _ := args[i].(slip.Symbol)
		if !strings.EqualFold(":binary-as", string(sym)) {
			slip.TypePanic(s, depth, "keyword", args[i], ":binary-as")
		}
		if len(args) <= i+1 {
			slip.ErrorPanic(s, depth, "%s is missing a value", sym)
		}
		binaryAs = binaryAsArg(s, args[i+1], depth)
	}
	inst := readerFlavor.MakeInstance().(*flavors.Instance)
	initReader(s, inst, string(path), binaryAs, depth)

	return inst
}

// initReader opens either a local file or, if the path is an HTTP or HTTPS
// URL, a remote file read with range requests.
func initReader(s *slip.Scope, inst *flavors.Instance, path string, binaryAs slip.Object, depth int) {
	var (
		pr  *file.Reader
		err error
	)
	if isURL(path) {
		var hr *httpReader
		if hr, err = newHTTPReader(path); err == nil {
			pr, err = file.NewParquetReader(hr)
		}
	} else {
		pr, err = file.OpenParquetFile(path, true)
	}
	var fr *pqarrow.FileReader
	if err == nil {
		fr, err = pqarrow.NewFileReader(pr, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	}
	if err != nil {
		slip.FilePanic(s, depth, slip.String(path), "%s", err)
	}
	inst.Any = fr
	inst.Let(slip.Symbol("filepath"), slip.String(path))
	if binaryAs != nil {
		inst.Let(slip.Symbol("binary-as"), binaryAs)
	}
}
