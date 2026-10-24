package render

import (
	"go/types"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/load"
)

type double struct {
	Package    string
	Imports    []importSpec
	Func       string
	FieldsType string
	Iface      string
	Struct     string
	Fields     []doubleField
}

// doubleField is a dependency the test supplies: Name on the fields struct, Field on the generated struct.
type doubleField struct {
	Name  string
	Field string
	Type  string
	// Nillable reports whether the double can tell that the value was never set.
	Nillable bool
}

func newDouble(out load.Output) double {
	s := newScope(out)
	s.imports.use("testing", "testing")

	d := double{
		Package:    out.Package.Name,
		Func:       config.DoubleName(out.Name),
		FieldsType: config.DoubleFields(out.Name),
		Struct:     s.structName,
	}
	fields := fieldNames(out.Methods)
	for i, m := range out.Methods {
		d.Fields = append(d.Fields, doubleField{
			Name:     m.Name,
			Field:    fields[i],
			Type:     s.typeString(m.Type),
			Nillable: nillable(m.Type),
		})
	}

	d.Iface = s.qualify(out.Target, out.Interface)
	d.Imports = s.imports.specs()
	return d
}

// nillable reports whether the zero value of t is nil.
func nillable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Signature, *types.Map, *types.Slice, *types.Chan:
		return true
	}
	return false
}
