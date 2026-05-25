// neo-moonchan @ 2024-7-29
// randomreader @ 240801

package randomreader

import (
	"crypto/rand"
	"math/big"
)

type RandomReader struct {
	dict string
}

func NewRandomReader(dict string) *RandomReader {
	return &RandomReader{dict: dict}
}

func (r *RandomReader) Read(p []byte) (int, error) {
	dictLength := big.NewInt(int64(len(r.dict)))

	for i := 0; i < len(p); i++ {
		// Generate a random index to select a character from the dictionary
		index, err := rand.Int(rand.Reader, dictLength)
		if err != nil {
			return i, err
		}
		p[i] = r.dict[index.Int64()]
	}

	return len(p), nil
}

var DefaultRandomReader = NewRandomReader("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func Read(p []byte) (int, error) {
	return DefaultRandomReader.Read(p)
}
