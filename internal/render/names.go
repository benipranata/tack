package render

import (
	"go/token"
	"go/types"
	"strconv"
	"unicode"
)

// namer hands out identifiers that are unique within one generated file.
type namer struct {
	used map[string]bool
	// universe also avoids predeclared identifiers such as len and nil, which a local would shadow.
	universe bool
}

func newNamer(universe bool, reserved ...string) *namer {
	n := &namer{used: map[string]bool{}, universe: universe}
	for _, r := range reserved {
		n.used[r] = true
	}
	return n
}

// take reserves name, or name with the lowest free numeric suffix when it is taken, a keyword or predeclared.
func (n *namer) take(name string) string {
	free := func(s string) bool {
		return !n.used[s] && !token.IsKeyword(s) && !(n.universe && types.Universe.Lookup(s) != nil)
	}
	got := name
	for i := 2; !free(got); i++ {
		got = name + strconv.Itoa(i)
	}
	n.used[got] = true
	return got
}

// lowerName makes an exported name unexported, treating a leading acronym as one word: DB gives db, HTTPClient gives httpClient.
func lowerName(s string) string {
	runes := []rune(s)
	n := 0
	for n < len(runes) && (unicode.IsUpper(runes[n]) || unicode.IsDigit(runes[n])) {
		n++
	}
	switch {
	case n == len(runes):
	case n > 1:
		n--
	case n == 0:
		return s
	}
	for i := range n {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
