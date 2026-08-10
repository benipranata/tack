package scan

import "go/types"

// qualifies reports whether sig is exactly func(context.Context) (T, func(), error),
// returning T if so.
func qualifies(sig *types.Signature) (types.Type, bool) {
	if sig.Variadic() {
		return nil, false
	}
	if sig.Params().Len() != 1 || !isContextContext(sig.Params().At(0).Type()) {
		return nil, false
	}
	if sig.Results().Len() != 3 {
		return nil, false
	}
	if !isEmptyFunc(sig.Results().At(1).Type()) {
		return nil, false
	}
	if !isBuiltinError(sig.Results().At(2).Type()) {
		return nil, false
	}
	return sig.Results().At(0).Type(), true
}

func isContextContext(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

func isEmptyFunc(t types.Type) bool {
	sig, ok := t.(*types.Signature)
	if !ok {
		return false
	}
	return !sig.Variadic() && sig.Params().Len() == 0 && sig.Results().Len() == 0
}

func isBuiltinError(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Pkg() == nil && named.Obj().Name() == "error"
}
