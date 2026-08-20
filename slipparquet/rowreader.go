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
	octets bool
}

// shardPlan is the part of one shard that a read covers.
type shardPlan struct {
	shard  int
	groups []int // nil for every row group
	skip   int64
}

type rowReader struct {
	sh     *shards
	ids    slip.Object
	plan   []shardPlan
	pi     int // index into plan of the shard being read
	cra    []*keyColReader
	format byte
	octets bool
	limit  int64 // negative for no limit
	count  int64
	depth  int
	scope  *slip.Scope
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
		value = arrayValue(kcr.chunks[0], kcr.ci, kcr.octets)
		kcr.ci++
	} else {
		kcr.chunks = kcr.chunks[1:]
		if 0 < len(kcr.chunks) {
			kcr.ccnt = kcr.chunks[0].Len()
			value = arrayValue(kcr.chunks[0], 0, kcr.octets)
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

// rowPlan works out which row groups of which shards hold the rows described
// by offset and limit. A dataset is read shard by shard in order so a global
// row number falls in exactly one of them.
func rowPlan(sh *shards, offset, limit int64) (plan []shardPlan) {
	if limit == 0 {
		return
	}
	remaining := limit
	var start int64
	for i, fr := range sh.frs {
		cnt := fr.ParquetReader().NumRows()
		if offset < start+cnt {
			local := offset - start
			if local < 0 {
				local = 0
			}
			groups, skip := rowGroupsFor(fr.ParquetReader(), local, remaining)
			plan = append(plan, shardPlan{shard: i, groups: groups, skip: skip})
			if 0 <= remaining {
				if avail := cnt - local; remaining <= avail {
					break
				} else {
					remaining -= avail
				}
			}
		}
		start += cnt
	}
	return
}

// colReader returns a reader for a field. A nil groups reads every row group
// which is the path taken when there is no offset or limit.
func colReader(ctx context.Context, fr *pqarrow.FileReader, i int, groups []int) *pqarrow.ColumnReader {
	if groups == nil {
		cr, _ := fr.GetColumn(ctx, i)
		return cr
	}
	cr, _ := fr.GetFieldReader(ctx, i, allLeaves(fr.ParquetReader()), groups)

	return cr
}

func newRowReader(
	s *slip.Scope,
	sh *shards,
	ids, format slip.Object,
	offset, limit int64,
	octets bool,
	depth int) *rowReader {
	rr := rowReader{sh: sh, ids: ids, limit: limit, octets: octets, depth: depth, scope: s}
	switch format {
	case slip.Symbol(":list"):
		rr.format = listRow
	case slip.Symbol(":assoc"):
		rr.format = assocRow
	default:
		slip.TypePanic(s, depth, "format", format, ":list", ":assoc")
	}
	// The columns asked for are checked against the schema now so a bad name
	// is reported even when the plan covers no shards at all.
	schem, err := sh.first().Schema()
	if err != nil {
		panic(err)
	}
	if tids, ok := ids.(slip.List); ok {
		for _, id := range tids {
			if findColIndex(s, schem, id, depth) < 0 {
				slip.ErrorPanic(s, depth, "column %s not found", id)
			}
		}
	} else if ids != nil {
		slip.TypePanic(s, depth, "column-ids", ids, "list", "nil")
	}
	rr.plan = rowPlan(sh, offset, limit)
	rr.openShard()

	return &rr
}

// openShard builds the column readers for the shard the plan is up to.
func (rr *rowReader) openShard() {
	rr.cra = nil
	if len(rr.plan) <= rr.pi {
		return
	}
	sp := rr.plan[rr.pi]
	fr := rr.sh.frs[sp.shard]
	schem, err := fr.Schema()
	if err != nil {
		panic(err)
	}
	switch tids := rr.ids.(type) {
	case nil:
		for i := 0; i < schem.NumFields(); i++ {
			rr.cra = append(rr.cra, &keyColReader{
				cr: colReader(rr.sh.context(), fr, i, sp.groups), key: schem.Field(i).Name,
				format: rr.format, octets: rr.octets,
			})
		}
	case slip.List:
		for _, id := range tids {
			idx := findColIndex(rr.scope, schem, id, rr.depth)
			if idx < 0 {
				slip.ErrorPanic(rr.scope, rr.depth, "column %s not found", id)
			}
			rr.cra = append(rr.cra, &keyColReader{
				cr: colReader(rr.sh.context(), fr, idx, sp.groups), key: schem.Field(idx).Name,
				format: rr.format, octets: rr.octets,
			})
		}
	}
	if 0 < sp.skip {
		for _, kcr := range rr.cra {
			kcr.skip(sp.skip)
		}
	}
}

func (rr *rowReader) next() (row slip.Object) {
	for {
		if 0 <= rr.limit && rr.limit <= rr.count {
			return
		}
		if len(rr.plan) <= rr.pi {
			return
		}
		if row = rr.nextInShard(); row != nil {
			rr.count++
			return
		}
		// The shard is spent so carry on with the next one in the plan.
		rr.pi++
		rr.openShard()
	}
}

// nextInShard returns the next row of the shard being read or nil when it has
// no more rows.
func (rr *rowReader) nextInShard() (row slip.Object) {
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
	if len(rr.cra) == 0 {
		row = nil
	}
	return
}

// findColIndex returns the field index of a column identified by index or
// name. A negative result means it was not found.
func findColIndex(s *slip.Scope, schem *arrow.Schema, id slip.Object, depth int) int {
	switch tid := id.(type) {
	case slip.Fixnum:
		if 0 <= tid && int(tid) < schem.NumFields() {
			return int(tid)
		}
	case slip.String:
		if fa := schem.FieldIndices(string(tid)); 0 < len(fa) {
			return fa[0]
		}
	case slip.Symbol:
		if fa := schem.FieldIndices(string(tid)); 0 < len(fa) {
			return fa[0]
		}
	default:
		slip.TypePanic(s, depth, "id", tid, "fixnum", "string", "symbol")
	}
	return -1
}

// ArrayValue returns a value in an arrow.Array. The validity of the index is
// checked before making this call.
//
// A null is nil. Without that check a null would be indistinguishable from the
// zero value of the column type, so a null int64 would read as 0 and a null
// date as the epoch.
//
// Binary values are strings. Use ArrayValueOctets to get them as octets.
func ArrayValue(aa arrow.Array, i int) slip.Object {
	return arrayValue(aa, i, false)
}

// ArrayValueOctets is ArrayValue except that binary values are octets rather
// than strings. Parquet does not distinguish between text and bytes unless the
// writer annotated the column so which one is wanted is up to the caller.
func ArrayValueOctets(aa arrow.Array, i int) slip.Object {
	return arrayValue(aa, i, true)
}

func arrayValue(aa arrow.Array, i int, octets bool) (value slip.Object) {
	if aa.IsNull(i) {
		return nil
	}
	switch tc := aa.(type) {
	case *array.Null: // type having no physical storage
		value = jsonToLisp(tc.Value(i), octets)
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
		value = binaryValue(tc.Value(i), octets)
	case *array.FixedSizeBinary: // a binary where each value occupies the same number of bytes
		value = binaryValue(tc.Value(i), octets)
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
	case *array.Dictionary: // values held once and referred to by index
		// Following the index into the dictionary keeps the value on the typed
		// path. Without this a dictionary column is encoded to JSON and parsed
		// back for every row, and categorical columns are usually dictionary
		// encoded so that is a common case rather than a rare one.
		value = arrayValue(tc.Dictionary(), tc.GetValueIndex(i), octets)

	default:
		// Includes array.Struct, array.List, and array.Map since there does
		// not appear to be any way to get the data for each item so the
		// horribly inefficient approach is taken of getting the JSON encoded
		// values and parsing. The issue is that the API is based on top level
		// columns and does not expose which elements in the column match up
		// with a row.
		value = jsonToLisp(tc.GetOneForMarshal(i), octets)
	}
	return
}

// binaryValue returns bytes as either a string or octets.
func binaryValue(b []byte, octets bool) slip.Object {
	if octets {
		return slip.Octets(b)
	}
	return slip.String(b)
}

// JSONToLisp convertes parquet JSON to lisp. It differs from
// slip.SimpleObject in that []any can also be representations of maps.
//
// Binary values are strings. Use JSONToLispOctets to get them as octets.
func JSONToLisp(val any) slip.Object {
	return jsonToLisp(val, false)
}

// JSONToLispOctets is JSONToLisp except that binary values are octets rather
// than strings. Nested columns such as a HuggingFace image feature carry their
// bytes through this path.
func JSONToLispOctets(val any) slip.Object {
	return jsonToLisp(val, true)
}

func jsonToLisp(val any, octets bool) (obj slip.Object) {
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
		obj = binaryValue(tv, octets)

	case time.Time:
		obj = slip.Time(tv)

	case []any:
		list := make(slip.List, 0, len(tv))
		for _, v := range tv {
			if m, ok := v.(map[string]any); ok && len(m) == 2 {
				if key, has := m["key"]; has {
					list = append(list, slip.List{
						jsonToLisp(key, octets),
						slip.Tail{Value: jsonToLisp(m["value"], octets)},
					})
					continue
				}
			}
			list = append(list, jsonToLisp(v, octets))
		}
		obj = list
	case map[string]any:
		list := make(slip.List, 0, len(tv))
		for k, v2 := range tv {
			list = append(list, slip.List{slip.String(k), slip.Tail{Value: jsonToLisp(v2, octets)}})
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
func rowCap(sh *shards, limit int64) int64 {
	cnt := sh.rowCount()
	if 0 <= limit && limit < cnt {
		return limit
	}
	return cnt
}
