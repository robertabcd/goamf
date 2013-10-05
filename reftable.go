package amf

import (
	"errors"
)

type refTable []interface{}

func (r *refTable) Get(i int) (interface{}, error) {
	if i >= 0 && i < len(*r) {
		return (*r)[i], nil
	}
	return nil, errors.New("refTable index out-of-bounds")
}

func (r *refTable) Add(v interface{}) {
	*r = append(*r, v)
}

func (r *refTable) Reserve() *interface{} {
	*r = append(*r, nil)
	return &(*r)[len(*r)-1]
}
