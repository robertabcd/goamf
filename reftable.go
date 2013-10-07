package amf

import (
	"fmt"
)

type refTable []interface{}

func (r *refTable) Get(i int) (interface{}, error) {
	if i >= 0 && i < len(*r) {
		return (*r)[i], nil
	}
	return nil, fmt.Errorf("refTable index out-of-bounds: %d, len: %d", i, len(*r))
}

func (r *refTable) Add(v interface{}) {
	*r = append(*r, v)
}

func (r *refTable) Reserve() *interface{} {
	*r = append(*r, nil)
	return &(*r)[len(*r)-1]
}

func (r *refTable) Len() int {
	return len(*r)
}
