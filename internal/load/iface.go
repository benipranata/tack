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

	var methods []Method
	var errs []error
	for i := range iface.NumMethods() {
		m := iface.Method(i)
		sig := m.Type().(*types.Signature)
		if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
			errs = append(errs, fmt.Errorf("method %s must take no arguments and return one value", m.Name()))
			continue
		}
		methods = append(methods, Method{Name: m.Name(), Type: sig.Results().At(0).Type()})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return methods, nil
}
