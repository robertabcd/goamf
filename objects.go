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
	Type   reflect.Type
	Traits *Traits
}

type Traits struct {
	ClassName string
	External  bool
	Dynamic   bool
	Nmemb     int
	Members   []string

	membersMap map[string]bool
}

func NewTraits(v interface{}, cls string, dynamic bool) *Traits {
	members, _ := getStructMembersAndDynamics(reflect.ValueOf(v))
	return &Traits{
		ClassName: cls,
		Nmemb:     len(members),
		Members:   members,
		Dynamic:   dynamic,
	}
}

func (t *Traits) RebuildMemberMap() {
	t.membersMap = make(map[string]bool)
	for _, key := range t.Members {
		t.membersMap[key] = true
	}
}

func (t *Traits) HasMember(key string) bool {
	if t.membersMap == nil {
		t.RebuildMemberMap()
	}
	_, ok := t.membersMap[key]
	return ok
}

type TraitsMapper struct {
	userDefinedTypes       map[string]*DefinedType
	reflectTypeToClassName map[reflect.Type]*DefinedType
}

func NewTraitsMapper() *TraitsMapper {
	return &TraitsMapper{
		userDefinedTypes:       make(map[string]*DefinedType),
		reflectTypeToClassName: make(map[reflect.Type]*DefinedType),
	}
}

func (tm *TraitsMapper) RegisterType(t interface{}, traits *Traits) {
	userType := &DefinedType{
		Type:   reflect.TypeOf(t),
		Traits: traits,
	}
	if traits.ClassName != "" {
		tm.userDefinedTypes[traits.ClassName] = userType
	}
	tm.reflectTypeToClassName[reflect.TypeOf(t)] = userType
}

func (tm *TraitsMapper) FindByClassName(cls string) *DefinedType {
	return tm.userDefinedTypes[cls]
}

func (tm *TraitsMapper) FindByReflectType(tp reflect.Type) *DefinedType {
	return tm.reflectTypeToClassName[tp]
}

var (
	DefaultTraitsMapper = NewTraitsMapper()
)

func RegisterType(t interface{}, traits *Traits) {
	DefaultTraitsMapper.RegisterType(t, traits)
}
