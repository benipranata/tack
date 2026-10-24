package render

import (
	"github.com/benipranata/tack/internal/load"
)

type constructor struct {
	Package string
	Imports []importSpec
	Func    string
	Iface   string
	Struct  string
	Steps   []step
	Fields  []field
}

// step creates one value; a provider that serves several methods gets one step.
type step struct {
	Var     string
	Cleanup string
	Call    string
	// Label names the provider in the error returned when it fails.
	Label string
}

// field is a field of the generated struct and the method that returns it.
type field struct {
	Name   string
	Method string
	Var    string
	Type   string
}

func newConstructor(out load.Output) constructor {
	s := newScope(out)
	s.imports.use("context", "context")
	if len(out.Methods) > 0 {
		s.imports.use("fmt", "fmt")
	}

	c := constructor{Package: out.Package.Name, Func: out.Name, Struct: s.structName}
	fields := fieldNames(out.Methods)
	varOf := map[load.Provider]string{}
	labels := map[string]int{}
	for i, m := range out.Methods {
		v, ok := varOf[m.Provider]
		if !ok {
			v = s.names.take(lowerName(m.Name))
			varOf[m.Provider] = v
			c.Steps = append(c.Steps, step{
				Var:     v,
				Cleanup: s.names.take(v + "Cleanup"),
				Call:    s.qualify(m.Provider.Package, m.Provider.Name),
				Label:   m.Provider.Name,
			})
			labels[m.Provider.Name]++
		}
		c.Fields = append(c.Fields, field{Name: fields[i], Method: m.Name, Var: v, Type: s.typeString(m.Type)})
	}
	for i, st := range c.Steps {
		if labels[st.Label] > 1 {
			c.Steps[i].Label = st.Call
		}
	}

	c.Iface = s.qualify(out.Target, out.Interface)
	c.Imports = s.imports.specs()
	return c
}

// fieldNames names the struct field of every method; the test double builds the same struct, so it must get the same names.
// A field may not share a name with a method, which an unexported method such as logger would.
func fieldNames(methods []load.Method) []string {
	reserved := make([]string, len(methods))
	for i, m := range methods {
		reserved[i] = m.Name
	}

	n := newNamer(false, reserved...)
	names := make([]string, len(methods))
	for i, m := range methods {
		names[i] = n.take(lowerName(m.Name))
	}
	return names
}
