package hash

import (
	"crypto/sha256"
	"hash"
)

func New() Hasher {
	a := sha256.New()
	b := sha256.New()
	return Hasher{a, b}
}

type Hasher struct {
	a, b hash.Hash
}

func (h Hasher) Write(b []byte) (int, error) {
	return h.a.Write(b)
}

func (h Hasher) Sum() []byte {
	h.b.Write(h.a.Sum([]byte{}))
	return h.b.Sum([]byte{})
}
