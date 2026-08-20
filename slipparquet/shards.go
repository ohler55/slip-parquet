// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

// shards are the parquet files that make up a reader. A dataset is often
// split into several files, a HuggingFace split being a list of them, and the
// reader presents them as one table. A reader opened on a single file simply
// has one shard.
type shards struct {
	frs   []*pqarrow.FileReader
	paths []string
}

// first returns the reader for the first shard. The schema, version, and
// metadata of a dataset are taken from it since every shard has to agree.
func (sh *shards) first() *pqarrow.FileReader {
	return sh.frs[0]
}

// rowCount is the number of rows in all of the shards.
func (sh *shards) rowCount() (cnt int64) {
	for _, fr := range sh.frs {
		cnt += fr.ParquetReader().NumRows()
	}
	return
}

// rowGroupCount is the number of row groups in all of the shards.
func (sh *shards) rowGroupCount() (cnt int) {
	for _, fr := range sh.frs {
		cnt += fr.ParquetReader().NumRowGroups()
	}
	return
}

// rowGroup maps a row group index across the whole dataset onto the shard
// holding it and the index within that shard. A negative shard means the
// index is out of range.
func (sh *shards) rowGroup(id int) (shard, local int) {
	if id < 0 {
		return -1, 0
	}
	for i, fr := range sh.frs {
		cnt := fr.ParquetReader().NumRowGroups()
		if id < cnt {
			return i, id
		}
		id -= cnt
	}
	return -1, 0
}

// close closes every shard.
func (sh *shards) close() {
	for _, fr := range sh.frs {
		_ = fr.ParquetReader().Close()
	}
}

// filepath is the path of the only shard or a list of paths when there are
// more than one.
func (sh *shards) filepath() slip.Object {
	if len(sh.paths) == 1 {
		return slip.String(sh.paths[0])
	}
	list := make(slip.List, len(sh.paths))
	for i, p := range sh.paths {
		list[i] = slip.String(p)
	}
	return list
}

// sameSchema reports whether a schema matches the first shard. Reading a
// dataset whose shards disagree would silently produce nonsense so the
// mismatch is caught when the reader is opened.
func sameSchema(a, b *arrow.Schema) bool {
	if a.NumFields() != b.NumFields() {
		return false
	}
	for i := 0; i < a.NumFields(); i++ {
		af := a.Field(i)
		bf := b.Field(i)
		if af.Name != bf.Name || !arrow.TypeEqual(af.Type, bf.Type) {
			return false
		}
	}
	return true
}

// readerShards returns the shards held by an instance.
func readerShards(inst *flavors.Instance) *shards {
	if inst == nil {
		return nil
	}
	sh, _ := inst.Any.(*shards)

	return sh
}

// selfShards returns the shards of the instance a method was sent to.
func selfShards(s *slip.Scope) (*flavors.Instance, *shards) {
	obj, _ := s.Get("self").(*flavors.Instance)

	return obj, readerShards(obj)
}
