# slip-parquet

A Parquet package for [SLIP](https://github.com/ohler55/slip).

## Building

The package is implemented as a Go plugin. All plugins require that
the version of the code pulling in the plugin and the plugin version
match. That that means is the plugin must be build with the same
version of SLIP in order to make use of the `require` LISP function to
load the parquet package. It's actually a bit mroe finicky than that
though. The build must be done agains the actual source code and not
simply putting the version in the go.mod requires.

First checkout the SLIP code for the tagged release. For example is a
directory parallel to this directory. Note that the go.mod file has a
`replace` directive to use the checked out code.

Next make the .so file:

```
make
```

The parquet.so file should now exist. It can be copied to the
directory that `*package-load-path*` is set to or by providing the
path to the directory containing the `parquet.so` file.

```lisp
(require 'parquet "my-package-directory")
```
