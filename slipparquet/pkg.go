// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
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
