// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet_test

import (
	"testing"

	"github.com/ohler55/slip"
	"github.com/ohler55/slip/sliptest"
)

// Key-value metadata is a plain parquet feature. The fixtures were written by
// four different applications and each stamps its own keys.
func TestMetadataSpark(t *testing.T) {
	scope := reader(t, "nested-maps.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-metadata reader))`,
		Expect: `("org.apache.spark.sql.parquet.row.metadata")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(stringp (cdr (assoc "org.apache.spark.sql.parquet.row.metadata" (parquet-metadata reader))))`,
		Expect: "t",
	}).Test(t)
}

func TestMetadataProtobuf(t *testing.T) {
	scope := reader(t, "binary.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-metadata reader))`,
		Expect: `("parquet.proto.descriptor" "writer.model.name" "parquet.proto.class")`,
	}).Test(t)
	(&sliptest.Function{
		Scope:  scope,
		Source: `(cdr (assoc "writer.model.name" (parquet-metadata reader)))`,
		Expect: `"protobuf"`,
	}).Test(t)
}

func TestMetadataArrow(t *testing.T) {
	scope := reader(t, "null-list.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(mapcar 'car (parquet-metadata reader))`,
		Expect: `("ARROW:schema")`,
	}).Test(t)
}

// A file with no key-value metadata gives nil rather than an empty list.
func TestMetadataNone(t *testing.T) {
	scope := reader(t, "primitive.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(parquet-metadata reader)`,
		Expect: "nil",
	}).Test(t)
}

func TestMetadataMethod(t *testing.T) {
	scope := reader(t, "binary.parquet")
	(&sliptest.Function{
		Scope:  scope,
		Source: `(equal (send reader :metadata) (parquet-metadata reader))`,
		Expect: "t",
	}).Test(t)
}

func TestMetadataBadReader(t *testing.T) {
	(&sliptest.Function{
		Source:    `(parquet-metadata 7)`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
	(&sliptest.Function{
		Source:    `(parquet-metadata (make-instance 'vanilla-flavor))`,
		PanicType: slip.Symbol("type-error"),
	}).Test(t)
}
