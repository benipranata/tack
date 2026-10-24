package load

import "go/types"

// Package is a package of the module; Dir is relative to the module root.
type Package struct {
	Dir  string
	Path string
	Name string
}

// Provider is a function that supplies a value, found in Package.
type Provider struct {
	Package Package
	Name    string
}

// Method is an interface method together with the provider that satisfies it.
type Method struct {
	Name     string
	Type     types.Type
	Provider Provider
	// local is the import path of the package that declares an unexported method, which alone can implement it.
	local string
}

// Output is one generated file with every method already bound to a provider.
type Output struct {
	Name      string
	Filename  string
	Package   Package
	Target    Package
	Interface string
	Methods   []Method
	// Test reports whether the output also gets a test double.
	Test bool
}
