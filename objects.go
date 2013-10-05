package amf

import (
	"encoding/json"
	"reflect"
	"strconv"
)

// Marker
type Marker uint8

const (
	MarkerUndefined Marker = iota
	MarkerNull
	MarkerFalse
	MarkerTrue
	MarkerInteger
	MarkerDouble
	MarkerString
	MarkerXMLDoc
	MarkerDate
	MarkerArray
	MarkerObject
	MarkerXML
	MarkerByteArray
)

// Predefined types
type AMF3Undefined struct{}
type AMF3Null struct{}

func (_ AMF3Null) MarshalJSON() ([]byte, error) {
	return json.Marshal(nil)
}

type TypedObject struct {
	Assoc map[string]interface{} `json:"assoc,omitempty"`
	Array []interface{}          `json:"array,omitempty"`
}

func (t TypedObject) MarshalJSON() ([]byte, error) {
	if len(t.Assoc) == 0 && len(t.Array) == 0 {
		return json.Marshal(map[string]interface{}{})
	} else if len(t.Assoc) == 0 {
		return json.Marshal(t.Array)
	} else if len(t.Array) == 0 {
		return json.Marshal(t.Assoc)
	} else {
		m := make(map[string]interface{})
		for k, v := range t.Assoc {
			m[k] = v
		}
		for i, v := range t.Array {
			m[strconv.Itoa(i)] = v
		}
		return json.Marshal(m)
	}
	panic("Not reached")
}

// User-defined type
type DefinedType struct {
	ClassName string
	Type      reflect.Type
	External  bool
}

type Traits struct {
	ClassName string
	External  bool
	Dynamic   bool
	Nmemb     int
	Members   []string
}

var (
	userdefinedTypes = make(map[string]*DefinedType)
)

func RegisterType(className string, t interface{}, external bool) {
	userdefinedTypes[className] = &DefinedType{
		ClassName: className,
		Type:      reflect.TypeOf(t),
		External:  external,
	}
}
