package a

var _ IA = (*A)(nil)

type A struct {
	X string
}

// Close implements [IA].
func (i *A) Close() error {
	return nil
}
