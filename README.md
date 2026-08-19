# slip-parquet

A Parquet package for [SLIP](https://github.com/ohler55/slip).

## Building

The package is implemented as a Go plugin. All plugins require that
the version of the code pulling in the plugin and the plugin version
match. That that means is the plugin must be build with the same
version of SLIP in order to make use of the `require` LISP function to
load the parquet package. It's actually a bit more finicky than that
though. The build must be done against the actual source code and not
simply putting the version in the go.mod requires.

First checkout the SLIP code for the tagged release. For example is a
directory parallel to this directory. Note that the go.mod file has a
`replace` directive to use the checked out code.

Next make the .so file:

```
make
```

The **parquet.so** file should now exist. It can be copied to the
directory that `*package-load-path*` is set to or by providing the
path to the directory containing the `parquet.so` file.

```lisp
(require 'parquet "my-package-directory")
```

## Usage

Open a file and ask it questions. The reader should be closed when done.

```lisp
(setq reader (parquet-open "sample.parquet"))

(send reader :row-count)     ;; => 15
(send reader :column-count)  ;; => 2
(send reader :created-by)    ;; => "parquet-go version 18.7.0"

(mapcar (lambda (f) (send f :name)) (send (send reader :schema) :fields))
;; => ("id" "name")

(send reader :close)
```

Rows come back either as a list of values in column order or as an assoc
list keyed by column name. A list of column names or indexes limits which
columns are read.

```lisp
(send reader :rows :list :limit 3)
;; => ((0 "row-00") (1 "row-01") (2 "row-02"))

(send reader :rows :assoc '("id") :offset 10 :limit 2)
;; => ((("id" . 10)) (("id" . 11)))
```

`:offset` and `:limit` only read the row groups holding the rows asked
for, so paging through a large file does not read all of it. To walk the
rows without building a list use `:each-row`, and to stop before the end
use a `block` and `return-from`.

```lisp
(send reader :each-row (lambda (row) (print row)) :list '("id") :limit 3)

(block walk
  (send reader :each-row
        (lambda (row) (when (< 100 (car row)) (return-from walk)) (print row))
        :list '("id")))
```

Every function has a matching method and the other way round, so
`(parquet-rows reader :list)` and `(send reader :rows :list)` are the
same except that the function form calls no daemons.

### Remote files

A file name that begins with `http://` or `https://` is read over the
network with range requests. Only the footer and the column chunks
actually used are fetched, so a multi gigabyte file can be examined
without downloading it. The server has to support range requests.

```lisp
(setq reader (parquet-open
  "https://huggingface.co/api/datasets/stanfordnlp/imdb/parquet/plain_text/train/0.parquet"))

(send reader :row-count)        ;; => 25000
(send reader :row-group-count)  ;; => 25

(send reader :rows :list '("label") :offset 12498 :limit 4)
;; => ((0) (0) (1) (1))
```

That last read pulled four rows from the middle of a 21MB file over the
network. Set the `HF_TOKEN` environment variable to reach a gated or
private HuggingFace dataset. The token is only ever sent to
`huggingface.co` and never to any other host.

### Looking before reading

Row group information and column statistics come from the file footer so
neither reads any column data. Both are a cheap way to size up a file,
and both work the same on a remote file.

```lisp
(send reader :row-group-count)
;; => 3

(send reader :row-group-info 1)
;; => (("rows" . 5) ("columns" . 2) ("byte-size" . 188) ("compressed-size" . 188))

(send reader :statistics "id")
;; => ((("row-group" . 0) ("num-values" . 5) ("min" . 0) ("max" . 4)
;;      ("null-count" . 0) ("distinct-count" . nil))
;;     ...)
```

Statistics are optional in a parquet file. When a writer recorded none
the `min` and `max` are nil. They are keyed by leaf column path so a
column nested in a struct is named `st.a` rather than `st`.

### File metadata

Any writer can put a map of strings in the file footer. Spark keeps its
SQL schema there, Arrow keeps a serialized schema, and a HuggingFace
dataset keeps its feature spec.

```lisp
(mapcar 'car (send reader :metadata))
;; => ("huggingface" "ARROW:schema")

(cdr (assoc "huggingface" (send reader :metadata)))
;; => "{\"info\": {\"features\": {\"label\": {\"names\": [\"neg\", \"pos\"], ...
```

Values are returned exactly as written so a value holding JSON is parsed
by the caller. That is what turns a `label` of 0 into `"neg"`.

### Nulls and binary columns

A null column value is nil. Note that a null and a false in a boolean
column are both nil since nil is false in lisp.

Parquet stores text and raw bytes in the same physical type and only
some writers record which is which. An unannotated column is a string by
default. For a column holding an image, audio, or another binary payload
open the reader with `:binary-as :octets`.

```lisp
(setq reader (parquet-open "images.parquet" :binary-as :octets))
```
