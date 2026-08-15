package tools

import (
	"testing"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

func TestWriteJSON(t *testing.T) {
	o := orderedmap.New()
	o.Set("1", []int{1, 2, 3})
	WriteJSONToFile("test.json", o)
}
