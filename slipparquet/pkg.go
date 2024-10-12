// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/ohler55/slip"
)

<<<<<<< HEAD:pkg.go
// Pkg is the parquet package.
var Pkg = slip.Package{
	Name:      "parquet",
	Nicknames: []string{"parquet"},
	Doc:       "Home of symbols defined for the parquet functions, variables, and constants.",
	PreSet:    slip.DefaultPreSet,
}

func init() {
	Pkg.Initialize(map[string]*slip.VarVal{})
=======
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
	Pkg.Initialize(nil)
	defReaderFlavor()
	defSchemaFlavor()

	slip.DefConstant(slip.Symbol("*parquet*"), &Pkg, "")

>>>>>>> develop:slipparquet/pkg.go
	slip.AddPackage(&Pkg)
	slip.UserPkg.Use(&Pkg)
}
