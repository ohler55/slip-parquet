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
