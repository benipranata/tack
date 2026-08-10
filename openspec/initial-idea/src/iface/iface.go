package iface

import (
	"case-01/src/a"
	"case-01/src/b"
	"case-01/src/c"
	"case-01/src/d"
)

type Iface interface {
	MethodA() a.IA
	B() b.IB
	Foo() *c.C
	BarClient() *d.D
}
