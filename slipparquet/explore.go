// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"io"
	"strconv"
	"strings"

	"github.com/ohler55/slip"
)

const (
	exploreLimit = 10 // rows shown when no limit is given
	exploreWidth = 40 // characters of a cell shown before it is cut
)

func init() {
	slip.Define(
		func(args slip.List) slip.Object {
			f := Explore{Function: slip.Function{Name: "parquet-explore", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-explore",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to look at.",
				},
				{Name: "&optional"},
				{
					Name: "column-ids",
					Type: "list",
					Text: "The columns to show. The default is all of them.",
				},
				{Name: "&key"},
				{
					Name: ":format",
					Type: "keyword",
					Text: "Either _:markdown_ which is the default, or _:tsv_.",
				},
				{
					Name: ":limit",
					Type: "fixnum",
					Text: "How many rows to show. The default is 10.",
				},
				{
					Name: ":offset",
					Type: "fixnum",
					Text: "The row to start at. The default is 0.",
				},
				{
					Name: ":width",
					Type: "fixnum",
					Text: `How many characters of a value to show before cutting it short.
The default is 40. A zero means do not cut values short at all.`,
				},
				{
					Name: ":null",
					Type: "string",
					Text: `What to show for a null. The default is an empty cell which
means a null and an empty string look the same. Set it to something like _"NULL"_ when telling
them apart matters.`,
				},
				{
					Name: ":summary",
					Type: "object",
					Text: `When not nil a line describing the file is written above the
table. It is not valid markdown or TSV so the default is off, which keeps the output ready to
paste elsewhere.`,
				},
				{
					Name: ":stream",
					Type: "output-stream",
					Text: "Where to write. The default is _*standard-output*_.",
				},
			},
			Return: "nil",
			Text: `__parquet-explore__ writes a few rows of a parquet reader as a table.

It is for looking at a file rather than for reading it. Values are cut short so that a column
of long text stays readable and only the rows asked for are read, so this is cheap even on a
large or remote file.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-explore reader)`,
				`(parquet-explore reader '("id" "name") :limit 3 :format :tsv)`,
				`(parquet-explore reader :null "NULL" :summary t)`,
			},
		}, &Pkg)
}

// Explore represents the parquet-explore function.
type Explore struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Explore) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 12)
	sh, octets := readerOctets(s, args, depth)
	opt := exploreOpts(s, args[1:], depth)

	// Only the rows shown are read so exploring a large file is cheap.
	rr := newRowReader(s, sh, opt.ids, slip.Symbol(":list"), opt.offset, opt.limit, octets, depth)
	rows := make([][]string, 0, opt.limit)
	for {
		row := rr.next()
		if row == nil {
			break
		}
		list, _ := row.(slip.List)
		cells := make([]string, len(list))
		for i, v := range list {
			cells[i] = cell(v, opt.width, opt.null)
		}
		rows = append(rows, cells)
	}
	head := exploreHead(s, sh, opt, depth)

	var b []byte
	if opt.summary {
		b = appendSummary(b, sh, opt, len(rows))
	}
	if opt.tsv {
		b = appendTSV(b, head, rows)
	} else {
		b = appendMarkdown(b, head, rows)
	}
	if _, err := opt.w.Write(b); err != nil {
		slip.StreamPanic(s, depth, opt.stream, "write failed: %s", err)
	}
	return slip.Novalue
}

// exploreKeys are the keywords parquet-explore takes.
var exploreKeys = []string{":format", ":limit", ":offset", ":width", ":null", ":summary", ":stream"}

// exploreOptions are the arguments of parquet-explore after the reader.
type exploreOptions struct {
	ids     slip.Object
	stream  slip.Stream
	w       io.Writer
	null    string
	offset  int64
	limit   int64
	width   int
	tsv     bool
	summary bool
}

// exploreOpts pulls the optional column-ids and the keywords.
func exploreOpts(s *slip.Scope, args slip.List, depth int) (opt exploreOptions) {
	opt.limit = exploreLimit
	opt.width = exploreWidth
	so := s.Get("*standard-output*")
	opt.stream, _ = so.(slip.Stream)
	opt.w, _ = so.(io.Writer)

	pos := 0
	if 0 < len(args) {
		if sym, ok := args[0].(slip.Symbol); !ok || !strings.HasPrefix(string(sym), ":") {
			opt.ids = args[0]
			pos = 1
		}
	}
	for ; pos < len(args); pos += 2 {
		sym, ok := args[pos].(slip.Symbol)
		if !ok {
			slip.TypePanic(s, depth, "keyword", args[pos], exploreKeys...)
		}
		if len(args) <= pos+1 {
			slip.ErrorPanic(s, depth, "%s is missing a value", sym)
		}
		val := args[pos+1]
		switch strings.ToLower(string(sym)) {
		case ":format":
			switch val {
			case slip.Symbol(":markdown"):
			case slip.Symbol(":tsv"):
				opt.tsv = true
			default:
				slip.TypePanic(s, depth, ":format", val, ":markdown", ":tsv")
			}
		case ":limit":
			opt.limit = int64(fixnumArg(s, ":limit", val, depth))
		case ":offset":
			opt.offset = int64(fixnumArg(s, ":offset", val, depth))
		case ":width":
			opt.width = fixnumArg(s, ":width", val, depth)
		case ":null":
			str, ok := val.(slip.String)
			if !ok {
				slip.TypePanic(s, depth, ":null", val, "string")
			}
			opt.null = string(str)
		case ":summary":
			opt.summary = val != nil
		case ":stream":
			if opt.w, ok = val.(io.Writer); !ok {
				slip.TypePanic(s, depth, ":stream", val, "output-stream")
			}
			opt.stream, _ = val.(slip.Stream)
		default:
			slip.TypePanic(s, depth, "keyword", sym, exploreKeys...)
		}
	}
	return
}

// fixnumArg checks that a keyword value is a non negative fixnum.
func fixnumArg(s *slip.Scope, use string, val slip.Object, depth int) int {
	num, ok := val.(slip.Fixnum)
	if !ok {
		slip.TypePanic(s, depth, use, val, "fixnum")
	}
	if num < 0 {
		slip.ErrorPanic(s, depth, "%s must not be negative", use)
	}
	return int(num)
}

// exploreHead is the column names of the table.
func exploreHead(s *slip.Scope, sh *shards, opt exploreOptions, depth int) []string {
	schem, err := sh.first().Schema()
	if err != nil {
		panic(err)
	}
	if ids, ok := opt.ids.(slip.List); ok {
		head := make([]string, len(ids))
		for i, id := range ids {
			idx := findColIndex(s, schem, id, depth)
			if idx < 0 {
				slip.ErrorPanic(s, depth, "column %s not found", id)
			}
			head[i] = schem.Field(idx).Name
		}
		return head
	}
	head := make([]string, schem.NumFields())
	for i := range head {
		head[i] = schem.Field(i).Name
	}
	return head
}

// cell is the printed form of a value cut to width. A string is shown without
// its quotes since a table is for reading rather than for reading back in.
func cell(v slip.Object, width int, null string) string {
	var str string
	switch tv := v.(type) {
	case nil:
		return null
	case slip.String:
		str = string(tv)
	default:
		str = string(slip.ObjectAppend(nil, v))
	}
	// A value holding a newline or a tab would break the table apart.
	str = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(str)
	if 0 < width && width < len([]rune(str)) {
		str = string([]rune(str)[:width-1]) + "…"
	}
	return str
}

// appendSummary describes the file above the table. It says how much of the
// file is being looked at since explore only ever shows a few rows.
func appendSummary(b []byte, sh *shards, opt exploreOptions, shown int) []byte {
	schem, err := sh.first().Schema()
	cols := 0
	if err == nil {
		cols = schem.NumFields()
	}
	b = append(b, plural(sh.rowCount(), "row")...)
	if 1 < len(sh.frs) {
		b = append(b, " in "...)
		b = append(b, plural(int64(len(sh.frs)), "shard")...)
	}
	b = append(b, ", "...)
	b = append(b, plural(int64(cols), "column")...)
	b = append(b, ", "...)
	b = append(b, plural(int64(sh.rowGroupCount()), "row group")...)
	if shown == 0 {
		b = append(b, ", showing none"...)
	} else {
		b = append(b, ", showing "...)
		b = strconv.AppendInt(b, opt.offset+1, 10)
		b = append(b, '-')
		b = strconv.AppendInt(b, opt.offset+int64(shown), 10)
	}
	return append(b, '\n', '\n')
}

// plural writes a count with its unit, adding an s when there is not just one.
func plural(cnt int64, unit string) string {
	if cnt == 1 {
		return "1 " + unit
	}
	return strconv.FormatInt(cnt, 10) + " " + unit + "s"
}

// appendMarkdown writes the rows as a markdown table.
func appendMarkdown(b []byte, head []string, rows [][]string) []byte {
	wide := make([]int, len(head))
	for i, h := range head {
		wide[i] = len([]rune(h))
	}
	for _, row := range rows {
		for i, c := range row {
			if i < len(wide) && wide[i] < len([]rune(c)) {
				wide[i] = len([]rune(c))
			}
		}
	}
	b = appendMDRow(b, head, wide)
	b = append(b, '|')
	for _, w := range wide {
		b = append(b, ' ')
		b = append(b, strings.Repeat("-", w)...)
		b = append(b, " |"...)
	}
	b = append(b, '\n')
	for _, row := range rows {
		b = appendMDRow(b, row, wide)
	}
	return b
}

func appendMDRow(b []byte, cells []string, wide []int) []byte {
	b = append(b, '|')
	for i, w := range wide {
		var c string
		if i < len(cells) {
			c = strings.ReplaceAll(cells[i], "|", "\\|")
		}
		b = append(b, ' ')
		b = append(b, c...)
		b = append(b, strings.Repeat(" ", w-len([]rune(c)))...)
		b = append(b, " |"...)
	}
	return append(b, '\n')
}

// appendTSV writes the rows as tab separated values.
func appendTSV(b []byte, head []string, rows [][]string) []byte {
	b = append(b, strings.Join(head, "\t")...)
	b = append(b, '\n')
	for _, row := range rows {
		b = append(b, strings.Join(row, "\t")...)
		b = append(b, '\n')
	}
	return b
}
