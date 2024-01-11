// Copyright (c) 2023, Peter Ohler, All rights reserved.

package main

import (
	"github.com/ohler55/slip"
)

// Pkg is the parquet package.
var Pkg = slip.Package{
	Name:      "parquet",
	Nicknames: []string{"parquet"},
	Doc:       "Home of symbols defined for the parquet functions, variables, and constants.",
	PreSet:    slip.DefaultPreSet,
}

func init() {
	Pkg.Initialize(map[string]*slip.VarVal{})
	slip.AddPackage(&Pkg)
	slip.UserPkg.Use(&Pkg)
	Pkg.Set("*parquet*", &Pkg)
}
