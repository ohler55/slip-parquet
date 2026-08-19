// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

var (
	// Pkg is the parquet package.
	Pkg = slip.Package{
		Name:      "parquet",
		Nicknames: []string{"parquet"},
		Doc:       "Home of symbols defined for the parquet functions, variables, and constants.",
		PreSet:    slip.DefaultPreSet,
	}
)

func init() {
	Pkg.Initialize(map[string]*slip.VarVal{
		"*parquet*": {
			Val:    &Pkg,
			Const:  true,
			Export: true,
			Doc:    `The parquet package.`,
		},
	})
	defReaderFlavor()
	defSchemaFlavor()

	slip.AddPackage(&Pkg)
	slip.UserPkg.Use(&Pkg)
}

// readerArg returns the file reader held by the first argument which must be a
// parquet-reader-flavor instance.
func readerArg(s *slip.Scope, args slip.List, depth int) *pqarrow.FileReader {
	inst, ok := args[0].(*flavors.Instance)
	if !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	var fr *pqarrow.FileReader
	if fr, ok = inst.Any.(*pqarrow.FileReader); !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	return fr
}

// rowGroupInfo returns the metadata for a row group as an assoc list. The
// index is checked against the number of row groups in the file.
func rowGroupInfo(s *slip.Scope, fr *pqarrow.FileReader, id slip.Object, depth int) slip.Object {
	num, ok := id.(slip.Fixnum)
	if !ok {
		slip.TypePanic(s, depth, "index", id, "fixnum")
	}
	pr := fr.ParquetReader()
	if num < 0 || slip.Fixnum(pr.NumRowGroups()) <= num {
		slip.ErrorPanic(s, depth, "row group %d is not in the range 0 to %d", num, pr.NumRowGroups()-1)
	}
	rg := pr.MetaData().RowGroup(int(num))

	return slip.List{
		slip.List{slip.String("rows"), slip.Tail{Value: slip.Fixnum(rg.NumRows())}},
		slip.List{slip.String("columns"), slip.Tail{Value: slip.Fixnum(rg.NumColumns())}},
		slip.List{slip.String("byte-size"), slip.Tail{Value: slip.Fixnum(rg.TotalByteSize())}},
		slip.List{slip.String("compressed-size"), slip.Tail{Value: slip.Fixnum(rg.TotalCompressedSize())}},
	}
}

// fileMetadata returns the parquet file key-value metadata as an assoc list.
//
// The metadata is a map of strings that any writer can put in the file footer.
// Spark stores its SQL schema there, Arrow stores a serialized schema, and
// HuggingFace stores the dataset feature spec including the class label names.
func fileMetadata(fr *pqarrow.FileReader) slip.Object {
	kv := fr.ParquetReader().MetaData().KeyValueMetadata()
	meta := make(slip.List, 0, kv.Len())
	for _, key := range kv.Keys() {
		var val slip.Object
		if v := kv.FindValue(key); v != nil {
			val = slip.String(*v)
		}
		meta = append(meta, slip.List{slip.String(key), slip.Tail{Value: val}})
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// isOctets reports whether a reader was opened with :binary-as :octets in
// which case binary column values are octets rather than strings.
func isOctets(inst *flavors.Instance) bool {
	return inst != nil && inst.Get("binary-as") == slip.Symbol(":octets")
}

// readerOctets returns the file reader held by the first argument along with
// how that reader represents binary values.
func readerOctets(s *slip.Scope, args slip.List, depth int) (*pqarrow.FileReader, bool) {
	fr := readerArg(s, args, depth)
	inst, _ := args[0].(*flavors.Instance)

	return fr, isOctets(inst)
}

// binaryAsArg checks a :binary-as value and returns it.
func binaryAsArg(s *slip.Scope, val slip.Object, depth int) slip.Object {
	switch val {
	case slip.Symbol(":string"), slip.Symbol(":octets"), nil:
		return val
	}
	slip.TypePanic(s, depth, ":binary-as", val, ":string", ":octets")

	return nil
}
