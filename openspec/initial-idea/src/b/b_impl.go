package b

func NewB() (IB, error) {
	return &B{}, nil
}

var _ IB = (*B)(nil)

type B struct{}

// Greeter implements [IB].
func (b *B) Greeter() error {
	return nil
}
