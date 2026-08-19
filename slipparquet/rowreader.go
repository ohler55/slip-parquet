// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/ojg/oj"
	"github.com/ohler55/slip"
)

type readerRowsCaller bool

const (
	listRow  = 'L'
	assocRow = 'A'
)

type keyColReader struct {
	key    string
	cr     *pqarrow.ColumnReader
	ac     *arrow.Chunked
	chunks []arrow.Array
	ci     int // chunk index
	ccnt   int // chunk len
	format byte
}

type rowReader struct {
	cra    []*keyColReader
	format byte
	limit  int64 // negative for no limit
	count  int64
}

func (kcr *keyColReader) next() (value slip.Object, done bool) {
begin:
	if kcr.ac == nil {
		var err error
		if kcr.ac, err = kcr.cr.NextBatch(batchSize); err != nil {
			panic(err)
		}
		if kcr.ac.Len() == 0 {
			return nil, true
		}
		kcr.chunks = kcr.ac.Chunks()
		kcr.ci = 0
		kcr.ccnt = kcr.chunks[0].Len()
	}
	if kcr.ci < kcr.ccnt {
		value = ArrayValue(kcr.chunks[0], kcr.ci)
		kcr.ci++
	} else {
		kcr.chunks = kcr.chunks[1:]
		if 0 < len(kcr.chunks) {
			kcr.ccnt = kcr.chunks[0].Len()
			value = ArrayValue(kcr.chunks[0], 0)
			kcr.ci = 1
		} else {
			kcr.ac = nil
			goto begin
		}
	}
	return
}

// skip discards n rows without turning them into lisp objects.
func (kcr *keyColReader) skip(n int64) {
	for 0 < n {
		size := n
		if batchSize < size {
			size = batchSize
		}
		ac, err := kcr.cr.NextBatch(size)
		if err != nil {
			panic(err)
		}
		if ac.Len() == 0 {
			break
		}
		n -= int64(ac.Len())
	}
}

// allLeaves returns every leaf column index which is what makes a field reader
// behave the same as a column reader while still limiting the row groups.
func allLeaves(pr *file.Reader) map[int]bool {
	cnt := pr.MetaData().Schema.NumColumns()
	leaves := make(map[int]bool, cnt)
	for i := 0; i < cnt; i++ {
		leaves[i] = true
	}
	return leaves
}

// rowGroupsFor returns the row groups holding the rows in the range described
// by offset and limit along with the number of rows to skip in the first of
// those groups. A nil groups means every row group is needed.
func rowGroupsFor(pr *file.Reader, offset, limit int64) (groups []int, skip int64) {
	if offset <= 0 && limit < 0 {
		return nil, 0
	}
	if limit == 0 {
		return []int{}, 0
	}
	md := pr.MetaData()
	var start, taken int64
	for i := 0; i < pr.NumRowGroups(); i++ {
		cnt := md.RowGroup(i).NumRows()
		if offset < start+cnt {
			if len(groups) == 0 {
				skip = offset - start
				taken = cnt - skip
			} else {
				taken += cnt
			}
			groups = append(groups, i)
			if 0 <= limit && limit <= taken {
				break
			}
		}
		start += cnt
	}
	if groups == nil {
		groups = []int{}
	}
	return
}

// colReader returns a reader for a field. A nil groups reads every row group
// which is the path taken when there is no offset or limit.
func colReader(fr *pqarrow.FileReader, i int, groups []int) *pqarrow.ColumnReader {
	if groups == nil {
		cr, _ := fr.GetColumn(context.Background(), i)
		return cr
	}
	cr, _ := fr.GetFieldReader(context.Background(), i, allLeaves(fr.ParquetReader()), groups)

	return cr
}

func newRowReader(
	s *slip.Scope,
	fr *pqarrow.FileReader,
	ids, format slip.Object,
	offset, limit int64,
	depth int) *rowReader {
	rr := rowReader{limit: limit}
	switch format {
	case slip.Symbol(":list"):
		rr.format = listRow
	case slip.Symbol(":assoc"):
		rr.format = assocRow
	default:
		slip.TypePanic(s, depth, "format", format, ":list", ":assoc")
	}
	schem, err := fr.Schema()
	if err != nil {
		panic(err)
	}
	groups, skip := rowGroupsFor(fr.ParquetReader(), offset, limit)
	switch tids := ids.(type) {
	case nil:
		for i := 0; i < schem.NumFields(); i++ {
			cr := colReader(fr, i, groups)
			rr.cra = append(rr.cra, &keyColReader{cr: cr, key: schem.Field(i).Name, format: rr.format})
		}
	case slip.List:
		for _, id := range tids {
			cr, key := findColReader(s, fr, schem, id, groups, depth)
			if cr == nil {
				slip.ErrorPanic(s, depth, "column %s not found", id)
			}
			rr.cra = append(rr.cra, &keyColReader{cr: cr, key: key, format: rr.format})
		}
	default:
		slip.TypePanic(s, depth, "column-ids", ids, "list", "nil")
	}
	if 0 < skip {
		for _, kcr := range rr.cra {
			kcr.skip(skip)
		}
	}
	return &rr
}

func (rr *rowReader) next() (row slip.Object) {
	if 0 <= rr.limit && rr.limit <= rr.count {
		return
	}
	switch rr.format {
	case listRow:
		rlist := make(slip.List, len(rr.cra))
		for i, kcr := range rr.cra {
			value, done := kcr.next()
			if done {
				return
			}
			rlist[i] = value
		}
		row = rlist
	case assocRow:
		alist := make(slip.List, len(rr.cra))
		for i, kcr := range rr.cra {
			value, done := kcr.next()
			if done {
				return
			}
			alist[i] = slip.List{slip.String(kcr.key), slip.Tail{Value: value}}
		}
		row = alist
	}
	if row != nil {
		rr.count++
	}
	return
}

func findColReader(
	s *slip.Scope,
	fr *pqarrow.FileReader,
	schem *arrow.Schema,
	id slip.Object,
	groups []int,
	depth int) (cr *pqarrow.ColumnReader, name string) {
	switch tid := id.(type) {
	case slip.Fixnum:
		if 0 <= tid && int(tid) < schem.NumFields() {
			cr = colReader(fr, int(tid), groups)
			name = schem.Field(int(tid)).Name
		}
	case slip.String:
		if fa := schem.FieldIndices(string(tid)); 0 < len(fa) {
			cr = colReader(fr, fa[0], groups)
			name = string(tid)
		}
	case slip.Symbol:
		if fa := schem.FieldIndices(string(tid)); 0 < len(fa) {
			cr = colReader(fr, fa[0], groups)
			name = string(tid)
		}
	default:
		slip.TypePanic(s, depth, "id", tid, "fixnum", "string", "symbol")
	}
	return
}

// ArrayValue returns a value in an arrow.Array. The validity of the index is
// checked before making this call.
//
// A null is nil. Without that check a null would be indistinguishable from the
// zero value of the column type, so a null int64 would read as 0 and a null
// date as the epoch.
func ArrayValue(aa arrow.Array, i int) (value slip.Object) {
	if aa.IsNull(i) {
		return nil
	}
	switch tc := aa.(type) {
	case *array.Null: // type having no physical storage
		value = JSONToLisp(tc.Value(i))
	case *array.Boolean: // is a 1 bit, LSB bit-packed ordering
		if tc.Value(i) {
			value = slip.True
		}
	case *array.Uint8: // a Unsigned 8-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Int8: // a Signed 8-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Uint16: // a Unsigned 16-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Int16: // a Signed 16-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Uint32: // a Unsigned 32-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Int32: // a Signed 32-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Uint64: // a Unsigned 64-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Int64: // a Signed 64-bit little-endian integer
		value = slip.Fixnum(tc.Value(i))
	case *array.Float16: // an 2-byte floating point value
		value = slip.SingleFloat(tc.Value(i).Float32())
	case *array.Float32: // an 4-byte floating point value
		value = slip.SingleFloat(tc.Value(i))
	case *array.Float64: // an 8-byte floating point value
		value = slip.DoubleFloat(tc.Value(i))
	case *array.String: // a UTF8 variable-length string
		value = slip.String(tc.Value(i))
	case *array.Binary: // a Variable-length byte type (no guarantee of UTF8-ness)
		value = slip.String(tc.Value(i))
	case *array.FixedSizeBinary: // a binary where each value occupies the same number of bytes
		value = slip.String(tc.Value(i))
	case *array.Date32: // int32 days since the UNIX epoch
		value = slip.Time(tc.Value(i).ToTime())
	case *array.Date64: // int64 milliseconds since the UNIX epoch
		value = slip.Time(tc.Value(i).ToTime())
	case *array.Timestamp: // an exact timestamp encoded with int64 since UNIX epoch Default unit millisecond
		tu := arrow.Millisecond
		if at, ok := tc.DataType().(*arrow.TimestampType); ok {
			tu = at.Unit
		}
		value = slip.Time(tc.Value(i).ToTime(tu))
	case *array.Time32: // a signed 32-bit integer, representing either seconds or milliseconds since midnight
		tu := arrow.Millisecond
		if at, ok := tc.DataType().(*arrow.Time32Type); ok {
			tu = at.Unit
		}
		value = slip.Time(tc.Value(i).ToTime(tu))
	case *array.Time64: // a signed 64-bit integer, representing either microseconds or nanoseconds since midnight
		tu := arrow.Nanosecond
		if at, ok := tc.DataType().(*arrow.Time64Type); ok {
			tu = at.Unit
		}
		value = slip.Time(tc.Value(i).ToTime(tu))
	case *array.Duration: // Measure of elapsed time in either seconds, milliseconds, microseconds or nanoseconds
		value = slip.Fixnum(tc.Value(i))

	default:
		// Includes array.Struct, array.List, and array.Map since there does
		// not appear to be any way to get the data for each item so the
		// horribly inefficient approach is taken of getting the JSON encoded
		// values and parsing. The issue is that the API is based on top level
		// columns and does not expose which elements in the column match up
		// with a row.
		value = JSONToLisp(tc.GetOneForMarshal(i))
	}
	return
}

// JSONToLisp convertes parquet JSON to lisp. It differs from
// slip.SimpleObject in that []any can also be representations of maps.
func JSONToLisp(val any) (obj slip.Object) {
	if raw, ok := val.(json.RawMessage); ok {
		val = oj.MustParse([]byte(raw))
	}
	switch tv := val.(type) {
	case slip.Object:
		obj = tv
	case bool:
		if tv {
			obj = slip.True
		}
	case int:
		obj = slip.Fixnum(tv)
	case int8:
		obj = slip.Fixnum(tv)
	case int16:
		obj = slip.Fixnum(tv)
	case int32:
		obj = slip.Fixnum(tv)
	case int64:
		obj = slip.Fixnum(tv)

	case uint:
		obj = slip.Fixnum(tv)
	case uint8:
		obj = slip.Fixnum(tv)
	case uint16:
		obj = slip.Fixnum(tv)
	case uint32:
		obj = slip.Fixnum(tv)
	case uint64:
		obj = slip.Fixnum(tv)

	case float32:
		obj = slip.SingleFloat(tv)
	case float64:
		obj = slip.DoubleFloat(tv)

	case string:
		obj = slip.String(tv)
	case []byte:
		obj = slip.String(tv)

	case time.Time:
		obj = slip.Time(tv)

	case []any:
		list := make(slip.List, 0, len(tv))
		for _, v := range tv {
			if m, ok := v.(map[string]any); ok && len(m) == 2 {
				if key, has := m["key"]; has {
					list = append(list, slip.List{JSONToLisp(key), slip.Tail{Value: JSONToLisp(m["value"])}})
					continue
				}
			}
			list = append(list, JSONToLisp(v))
		}
		obj = list
	case map[string]any:
		list := make(slip.List, 0, len(tv))
		for k, v2 := range tv {
			list = append(list, slip.List{slip.String(k), slip.Tail{Value: JSONToLisp(v2)}})
		}
		obj = list
	}
	return
}

// parseRowOpts pulls the optional column-ids and the :offset and :limit
// keywords from the arguments that follow the fixed arguments of :rows and
// :each-row. The column-ids are a list so a keyword in that position means
// the column-ids were left out.
func parseRowOpts(s *slip.Scope, args slip.List, depth int) (ids slip.Object, offset, limit int64) {
	limit = -1
	pos := 0
	if 0 < len(args) {
		if sym, ok := args[0].(slip.Symbol); !ok || !strings.HasPrefix(string(sym), ":") {
			ids = args[0]
			pos = 1
		}
	}
	for ; pos < len(args); pos += 2 {
		sym, ok := args[pos].(slip.Symbol)
		if !ok {
			slip.TypePanic(s, depth, "keyword", args[pos], ":offset", ":limit")
		}
		if len(args) <= pos+1 {
			slip.ErrorPanic(s, depth, "%s is missing a value", sym)
		}
		num, ok := args[pos+1].(slip.Fixnum)
		if !ok {
			slip.TypePanic(s, depth, string(sym), args[pos+1], "fixnum")
		}
		if num < 0 {
			slip.ErrorPanic(s, depth, "%s must not be negative", sym)
		}
		switch strings.ToLower(string(sym)) {
		case ":offset":
			offset = int64(num)
		case ":limit":
			limit = int64(num)
		default:
			slip.TypePanic(s, depth, "keyword", sym, ":offset", ":limit")
		}
	}
	return
}

// rowCap is the capacity to allocate for a row list.
func rowCap(fr *pqarrow.FileReader, limit int64) int64 {
	cnt := fr.ParquetReader().NumRows()
	if 0 <= limit && limit < cnt {
		return limit
	}
	return cnt
}
