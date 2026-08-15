// azure

package myiter

import "github.com/Hana-ame/api-pack/tools/orderedmap"

type SliceIter []any

func (i SliceIter) Iter(f func(k, v any) bool) {
	for k, v := range i {
		shouldStop := f(k, v)
		if shouldStop {
			return
		}
	}
}

type MapIter map[any]any

func (i MapIter) Iter(f func(k, v any) bool) {
	for k, v := range i {
		shouldStop := f(k, v)
		if shouldStop {
			return
		}
	}
}

type OrderedMapIter struct {
	*orderedmap.OrderedMap
}

func (i *OrderedMapIter) Iter(f func(k, v any) bool) {
	for _, k := range i.Keys() {
		v, isExist := i.Get(k)
		if isExist {
			shouldStop := f(k, v)
			if shouldStop {
				return
			}
		}
	}
}

type iter interface {
	Iter(f func(k, v any) (shouldStop bool))
}

func NewIter(o any) iter {
	switch v := o.(type) {
	case []any:
		return SliceIter(v)
	case map[any]any:
		return MapIter(v)
	case *orderedmap.OrderedMap:
		return &OrderedMapIter{v}
	}
	return nil
}
