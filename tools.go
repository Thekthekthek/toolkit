package toolkit

import (
	"crypto/rand"
)

const randomStringSource = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_+"

// Tools is a struct that contains some useful functions
type Tools struct{}

// The `func (t *Tools) randomStringSource(n int) string {` is a method of the `Tools` struct. It
// generates a random string of length `n` using characters from the `randomStringSource` constant. The
// method uses the `crypto/rand` package to generate random numbers and selects characters from the
// `randomStringSource` based on the generated numbers. The generated string is then returned.
func (t *Tools) RandomStringSource(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}
