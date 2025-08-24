// Copyright (c) 2023, Peter Ohler, All rights reserved.

package slipparquet

import (
	"io"

	"github.com/apache/arrow/go/v15/parquet/pqarrow"
	"github.com/apache/arrow/go/v15/parquet/schema"
	"github.com/ohler55/slip"
	"github.com/ohler55/slip/pkg/flavors"
)

var schemaFlavor *flavors.Flavor

// Both the schema flavor and the parquet-schema function are defined in this
// file.

func defSchemaFlavor() {
	schemaFlavor = flavors.DefFlavor("parquet-schema-flavor",
		map[string]slip.Object{},
		nil,
		slip.List{
			slip.List{
				slip.Symbol(":documentation"),
				slip.String(`A parquet schema represents a parquet schema node.`),
			},
		},
		&Pkg,
	)
	schemaFlavor.Final = true
	schemaFlavor.DefMethod(":name", "", schemaNameCaller(true))
	schemaFlavor.DefMethod(":type", "", schemaTypeCaller(true))
	schemaFlavor.DefMethod(":field-id", "", schemaFieldIDCaller(true))
	schemaFlavor.DefMethod(":repetition", "", schemaRepetitionCaller(true))
	schemaFlavor.DefMethod(":logical-type", "", schemaLogicalTypeCaller(true))
	schemaFlavor.DefMethod(":converted-type", "", schemaConvertedTypeCaller(true))
	schemaFlavor.DefMethod(":path", "", schemaPathCaller(true))
	schemaFlavor.DefMethod(":type-length", "", schemaTypeLengthCaller(true))
	schemaFlavor.DefMethod(":precision", "", schemaPrecisionCaller(true))
	schemaFlavor.DefMethod(":scale", "", schemaScaleCaller(true))
	schemaFlavor.DefMethod(":write", "", schemaWriteCaller(true))
	schemaFlavor.DefMethod(":parent", "", schemaParentCaller(true))
	schemaFlavor.DefMethod(":fields", "", schemaFieldsCaller(true))
	schemaFlavor.DefMethod(":find", "", schemaFindCaller(true))

	slip.Define(
		func(args slip.List) slip.Object {
			f := Schema{Function: slip.Function{Name: "parquet-schema", Args: args}}
			f.Self = &f
			return &f
		},
		&slip.FuncDoc{
			Name: "parquet-schema",
			Args: []*slip.DocArg{
				{
					Name: "reader",
					Type: "parquet-reader-flavor instance",
					Text: "The parquet reader to get the schema of.",
				},
			},
			Return: "nil",
			Text: `__parquet-schema__ returns the schema a parquet reader.

This is the same as sending _:schema_ to an instance of the _parquet-reader-flavor_ except
that none of the _:schema_ daemons are called.`,
			Examples: []string{
				`(setq reader (parquet-open "sample.parquet"))`,
				`(parquet-schema reader) => nil`,
			},
		}, &Pkg)
}

// Schema represents the parquet-schema function.
type Schema struct {
	slip.Function
}

// Call the function with the arguments provided.
func (f *Schema) Call(s *slip.Scope, args slip.List, depth int) slip.Object {
	slip.CheckArgCount(s, depth, f, args, 1, 1)
	inst, ok := args[0].(*flavors.Instance)
	if !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	var fr *pqarrow.FileReader
	if fr, ok = inst.Any.(*pqarrow.FileReader); !ok {
		slip.TypePanic(s, depth, "reader", args[0], "parquet-reader-flavor instance")
	}
	return makeSchemaInstance(fr.ParquetReader().MetaData().Schema.Root())
}

// SchemaFlavor returns the schema flavor.
func SchemaFlavor() *flavors.Flavor {
	return schemaFlavor
}

type schemaNameCaller bool

func (caller schemaNameCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		result = slip.String(node.Name())
	}
	return
}

func (caller schemaNameCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":name",
		Text:   "Returns the name of the schema node.",
		Return: "string",
	}
}

type schemaTypeCaller bool

func (caller schemaTypeCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		if node.Type() == schema.Group {
			result = slip.Symbol(":group")
		} else {
			result = slip.Symbol(":primitive")
		}
	}
	return
}

func (caller schemaTypeCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":type",
		Text:   "Returns the type of the schema node which can be :group or :primitive.",
		Return: "string",
	}
}

type schemaFieldIDCaller bool

func (caller schemaFieldIDCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		if fid := node.FieldID(); 0 <= fid {
			result = slip.Fixnum(fid)
		}
	}
	return
}

func (caller schemaFieldIDCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":field-id",
		Text:   "Returns the field ID of the schema node if there is a non negative ID.",
		Return: "fixnum|nil",
	}
}

type schemaLogicalTypeCaller bool

func (caller schemaLogicalTypeCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		if lt := node.LogicalType(); lt.IsValid() && !lt.IsNone() {
			result = slip.String(lt.String())
		}
	}
	return
}

func (caller schemaLogicalTypeCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":logical-type",
		Text:   "Returns the logical type of the schema node.",
		Return: "string",
	}
}

type schemaConvertedTypeCaller bool

func (caller schemaConvertedTypeCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		if ct := node.ConvertedType(); 0 <= ct {
			result = slip.String(ct.String())
		}
	}
	return
}

func (caller schemaConvertedTypeCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":converted-type",
		Text:   "Returns the converted type of the schema node.",
		Return: "string",
	}
}

type schemaPathCaller bool

func (caller schemaPathCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		result = slip.String(node.Path())
	}
	return
}

func (caller schemaPathCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":path",
		Text:   "Returns the path of the schema node.",
		Return: "string",
	}
}

type hasLength interface {
	TypeLength() int
}

type schemaTypeLengthCaller bool

func (caller schemaTypeLengthCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if hl, ok := obj.Any.(hasLength); ok {
		if tl := hl.TypeLength(); 0 < tl {
			result = slip.Fixnum(tl)
		}
	}
	return
}

func (caller schemaTypeLengthCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":type-length",
		Text:   "Returns the type length of the schema node if the type has length of _nil_ otherwise.",
		Return: "fixnum|nil",
	}
}

type schemaPrecisionCaller bool

func (caller schemaPrecisionCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if pn, ok := obj.Any.(*schema.PrimitiveNode); ok {
		result = slip.Fixnum(pn.DecimalMetadata().Precision)
	}
	return
}

func (caller schemaPrecisionCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":precision",
		Text:   "Returns the type precision of the schema node if the type has precision of _nil_ otherwise.",
		Return: "fixnum|nil",
	}
}

type schemaScaleCaller bool

func (caller schemaScaleCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if pn, ok := obj.Any.(*schema.PrimitiveNode); ok {
		result = slip.Fixnum(pn.DecimalMetadata().Scale)
	}
	return
}

func (caller schemaScaleCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":scale",
		Text:   "Returns the type scale of the schema node if the type has scale of _nil_ otherwise.",
		Return: "fixnum|nil",
	}
}

type schemaRepetitionCaller bool

func (caller schemaRepetitionCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		// Internal constants not visible so just use the integer values.
		switch node.RepetitionType() {
		case 0:
			result = slip.Symbol(":required")
		case 1:
			result = slip.Symbol(":optional")
		case 2:
			result = slip.Symbol(":repeated")
		}
	}
	return
}

func (caller schemaRepetitionCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":repetition",
		Text:   "Returns the repetition type of the schema node which can be :required, :optional, or :repeated.",
		Return: "keyword",
	}
}

type schemaWriteCaller bool

func (caller schemaWriteCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if len(args) != 1 {
		slip.MethodArgChoicePanic(s, depth, obj, "destination", len(args), "1")
	}
	if node, ok := obj.Any.(schema.Node); ok {
		sa := schemaAppender{pad: 2}
		node.Visit(&sa)
		switch ta := args[0].(type) {
		case nil:
			result = slip.String(sa.buf)
		case io.Writer:
			if _, err := ta.Write(sa.buf); err != nil {
				panic(err)
			}
		default:
			if ta == slip.True {
				if _, err := s.Get(slip.Symbol("*standard-output*")).(io.Writer).Write(sa.buf); err != nil {
					panic(err)
				}
			} else {
				slip.TypePanic(s, depth, "destination", ta, "nil", "t", "output-stream")
			}
		}
	}
	return
}

func (caller schemaWriteCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":write",
		Text: "Writes a line as if it were part of a schema file.",
		Args: []*slip.DocArg{
			{
				Name: "destination",
				Type: "output-stream|t|nil",
				Text: `either an _output-stream_, _t_, or _nil_. If an _output-stream_ then
write to it. If _t_ then write to _*standard-output*_. If _nil_ then return a string.`,
			},
		},
		Return: "string|nil",
	}
}

type schemaParentCaller bool

func (caller schemaParentCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		if parent := node.Parent(); parent != nil {
			result = makeSchemaInstance(parent)
		}
	}
	return
}

func (caller schemaParentCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":parent",
		Text:   "Returns the parent of the schema node if it has one.",
		Return: "instance",
	}
}

type schemaFieldsCaller bool

func (caller schemaFieldsCaller) Call(s *slip.Scope, args slip.List, _ int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok && node.Type() == schema.Group {
		g := node.(*schema.GroupNode)
		cnt := g.NumFields()
		fields := make(slip.List, cnt)
		for i := 0; i < cnt; i++ {
			fields[i] = makeSchemaInstance(g.Field(i))
		}
		result = fields
	}
	return
}

func (caller schemaFieldsCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name:   ":fields",
		Text:   "Returns the fields of the schema node if it is a group node.",
		Return: "list",
	}
}

type schemaFindCaller bool

func (caller schemaFindCaller) Call(s *slip.Scope, args slip.List, depth int) (result slip.Object) {
	obj := s.Get("self").(*flavors.Instance)
	if node, ok := obj.Any.(schema.Node); ok {
		path, ok := args[0].(slip.List)
		if !ok {
			path = args
		}
		for _, x := range path {
			var gn *schema.GroupNode
			if gn, ok = node.(*schema.GroupNode); !ok {
				return
			}
			switch tx := x.(type) {
			case slip.Fixnum:
				if tx < 0 || gn.NumFields() <= int(tx) {
					return
				}
				node = gn.Field(int(tx))
			case slip.String:
				i := gn.FieldIndexByName(string(tx))
				if i < 0 {
					return
				}
				node = gn.Field(i)
			case slip.Symbol:
				i := gn.FieldIndexByName(string(tx))
				if i < 0 {
					return
				}
				node = gn.Field(i)
			default:
				slip.TypePanic(s, depth, "path member", tx, "fixnum", "string", "symbol")
			}
		}
		result = makeSchemaInstance(node)
	}
	return
}

func (caller schemaFindCaller) FuncDocs() *slip.FuncDoc {
	return &slip.FuncDoc{
		Name: ":find",
		Text: `Finds a field schema in the schema based on the _path_ where a _fixnum_ indicates the index
of the child field and a _string_ inicates the name of a child field.`,
		Args: []*slip.DocArg{
			{Name: "&rest"},
			{
				Name: "path",
				Type: "fixnum|string|list",
				Text: `Either a _fixnum_, _string_, or a list of _fixnum_ and _strings_ that describe a
path to a child in the schema.`,
			},
		},
		Return: "instance|nil",
	}
}

func makeSchemaInstance(node schema.Node) (inst *flavors.Instance) {
	inst = schemaFlavor.MakeInstance().(*flavors.Instance)
	inst.Any = node
	return
}
