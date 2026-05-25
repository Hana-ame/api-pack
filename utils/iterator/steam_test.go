package iterator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jucardi/go-streams/v2/streams"
)

func TestXxx(t *testing.T) {

	var fruitArray = []string{"peach", "apple", "pear", "plum", "pineapple", "banana", "kiwi", "orange"}

	fruitsThatStartWithP := streams.

		// Creates a stream from the given array
		From[string](fruitArray).

		// Adds a filter for strings that start with 'p'
		Filter(func(v string) bool {
			return strings.HasPrefix(v, "p")
		}).

		// Sorts alphabetically
		Sort(strings.Compare).

		// Converts back to an array
		ToArray()

	fmt.Println(fruitsThatStartWithP)
}
