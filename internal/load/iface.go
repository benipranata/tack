package load

import (
	"errors"
	"fmt"
	"go/types"
)

// interfaceMethods returns the methods of the named interface; each must take no arguments and return one value.
func interfaceMethods(pkg *types.Package, name string) ([]Method, error) {
	obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("type %q not found in package %s", name, pkg.Path())
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("%q is not an interface", name)
	}
	if isGeneric(obj.Type()) {
		return nil, fmt.Errorf("%q is generic, which is not supported; name an instantiation such as type Int = %s[int]", name, name)
	}

	var methods []Method
	var errs []error
	for i := range iface.NumMethods() {
		m := iface.Method(i)
		sig := m.Type().(*types.Signature)
		if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
			errs = append(errs, fmt.Errorf("method %s must take no arguments and return one value", m.Name()))
			continue
		}
		method := Method{Name: m.Name(), Type: sig.Results().At(0).Type()}
		if !m.Exported() {
			method.local = m.Pkg().Path()
		}
		methods = append(methods, method)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return methods, nil
}

// checkImplementable reports each unexported method that a type in pkgPath cannot implement.
func checkImplementable(methods []Method, pkgPath string) []error {
	var errs []error
	for _, m := range methods {
		if m.local != "" && m.local != pkgPath {
			errs = append(errs, fmt.Errorf("%s(): unexported, so it can only be implemented in package %s", m.Name, m.local))
		}
	}
	return errs
}

// isGeneric reports whether t still has type parameters; an alias of an instantiation does not.
func isGeneric(t types.Type) bool {
	switch t := t.(type) {
	case *types.Named:
		return t.TypeParams().Len() > 0
	case *types.Alias:
		return t.TypeParams().Len() > 0
	}
	return false
}
