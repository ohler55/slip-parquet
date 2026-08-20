// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"context"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
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
					Text: `The parquet file to read. Either a local path or an HTTP or HTTPS URL.

A list of files is read as one table which is how a dataset split into shards is opened. The
shards must all have the same schema.`,
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
				{
					Name: ":timeout",
					Type: "fixnum|float",
					Text: `How long to wait, in seconds, for a remote server to answer. The default is 30.

The timeout covers connecting and waiting for the response headers. It deliberately does not
bound the transfer of the body since a single column chunk can legitimately take a long time
to arrive over a slow link. Has no effect on a local file.`,
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
				`(parquet-open '("part-0.parquet" "part-1.parquet")) ;; read as one table`,
				`(parquet-open "https://host/big.parquet" :timeout 5)`,
			},
		}, &Pkg)
}

// Open represents the parquet-open function.
type Open struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Open) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 5)
	paths := pathsArg(s, args[0], "file", depth)
	var binaryAs, timeout slip.Object
	for i := 1; i < len(args); i += 2 {
		sym, _ := args[i].(slip.Symbol)
		if len(args) <= i+1 {
			slip.ErrorPanic(s, depth, "%s is missing a value", sym)
		}
		switch {
		case strings.EqualFold(":binary-as", string(sym)):
			binaryAs = binaryAsArg(s, args[i+1], depth)
		case strings.EqualFold(":timeout", string(sym)):
			timeout = args[i+1]
		default:
			slip.TypePanic(s, depth, "keyword", args[i], ":binary-as", ":timeout")
		}
	}
	inst := readerFlavor.MakeInstance().(*flavors.Instance)
	initReader(s, inst, paths, binaryAs, timeout, depth)

	return inst
}

// initReader opens the files of a reader. Each is either a local file or, if
// the path is an HTTP or HTTPS URL, a remote file read with range requests.
func initReader(
	s *slip.Scope,
	inst *flavors.Instance,
	paths []string,
	binaryAs, timeout slip.Object,
	depth int) {
	if len(paths) == 0 {
		slip.ErrorPanic(s, depth, "no parquet file to open")
	}
	// The context lives as long as the reader rather than carrying a deadline
	// so a long browsing session is never cut short. Closing the reader
	// cancels it which aborts a read waiting on a slow server.
	ctx, cancel := context.WithCancel(context.Background())
	sh := shards{paths: paths, ctx: ctx, cancel: cancel}
	wait := timeoutArg(s, timeout, depth)
	for _, path := range paths {
		sh.frs = append(sh.frs, openOne(s, ctx, path, wait, depth))
	}
	// Rows from shards that do not agree on a schema can not be put in the
	// same table so the mismatch is caught here rather than mid read.
	base, err := sh.first().Schema()
	if err != nil {
		slip.FilePanic(s, depth, slip.String(paths[0]), "%s", err)
	}
	for i := 1; i < len(sh.frs); i++ {
		var sc *arrow.Schema
		if sc, err = sh.frs[i].Schema(); err != nil {
			slip.FilePanic(s, depth, slip.String(paths[i]), "%s", err)
		}
		if !sameSchema(base, sc) {
			sh.close()
			slip.ErrorPanic(s, depth, "%s does not have the same schema as %s", paths[i], paths[0])
		}
	}
	inst.Any = &sh
	inst.Let(slip.Symbol("filepath"), sh.filepath())
	if binaryAs != nil {
		inst.Let(slip.Symbol("binary-as"), binaryAs)
	}
	if timeout != nil {
		inst.Let(slip.Symbol("timeout"), timeout)
	}
}

// timeoutArg turns a :timeout of seconds into a duration. A missing or zero
// timeout means the default.
func timeoutArg(s *slip.Scope, obj slip.Object, depth int) time.Duration {
	var secs float64
	switch tv := obj.(type) {
	case nil:
		return 0
	case slip.Fixnum:
		secs = float64(tv)
	case slip.SingleFloat:
		secs = float64(tv)
	case slip.DoubleFloat:
		secs = float64(tv)
	default:
		slip.TypePanic(s, depth, ":timeout", obj, "fixnum", "float")
	}
	if secs < 0 {
		slip.ErrorPanic(s, depth, ":timeout must not be negative")
	}
	return time.Duration(secs * float64(time.Second))
}

// openOne opens a single parquet file.
func openOne(
	s *slip.Scope,
	ctx context.Context,
	path string,
	wait time.Duration,
	depth int) *pqarrow.FileReader {
	var (
		pr  *file.Reader
		err error
	)
	if isURL(path) {
		var hr *httpReader
		if hr, err = newHTTPReader(ctx, path, wait); err == nil {
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
	return fr
}

// pathsArg accepts either a single file or a list of the files that make up a
// dataset.
func pathsArg(s *slip.Scope, obj slip.Object, use string, depth int) []string {
	switch tv := obj.(type) {
	case slip.String:
		return []string{string(tv)}
	case slip.List:
		paths := make([]string, len(tv))
		for i, v := range tv {
			ss, ok := v.(slip.String)
			if !ok {
				slip.TypePanic(s, depth, use, v, "string")
			}
			paths[i] = string(ss)
		}
		return paths
	}
	slip.TypePanic(s, depth, use, obj, "string", "list")

	return nil
}
