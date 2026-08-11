package state

type Store struct {
	Env string
}

type State interface {
	Store() *Store
}
