package amf

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type DefinedType struct {
	ClassName string
	Type      reflect.Type
	External  bool
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

type Decoder struct {
	reader io.Reader

	stringRefs refTable
	objectRefs refTable
	traitsRefs refTable
}

type Traits struct {
	ClassName string
	External  bool
	Dynamic   bool
	Nmemb     int
	Members   []string
}

func NewDecoder(reader io.Reader) *Decoder {
	return &Decoder{
		reader: reader,
	}
}

func (d *Decoder) Decode(objptr interface{}) error {
	v := reflect.ValueOf(objptr)
	return d.readReflectValue(&v)
}

func (d *Decoder) readUInt8() (uint8, error) {
	b := []byte{0}
	_, err := d.reader.Read(b)
	return b[0], err
}

func (d *Decoder) readUInt29() (uint32, error) {
	var v uint32
	for j := 0; j < 3; j++ {
		b, err := d.readUInt8()
		if err != nil {
			return 0, err
		}
		v = (v << 7) | (uint32(b) & 0x7F)
		if (b & 0x80) != 0x80 {
			return v, nil
		}
	}
	b, err := d.readUInt8()
	if err != nil {
		return 0, err
	}
	v = (v << 8) | uint32(b)
	return v, err
}

func (d *Decoder) readString() (string, error) {
	length, err := d.readUInt29()
	if err != nil {
		return "", err
	}
	if (length & 1) == 0 {
		if v, err := d.stringRefs.Get(int(length >> 1)); err != nil {
			return "", nil
		} else {
			return v.(string), nil
		}
	}
	length >>= 1
	buf := make([]byte, length, length)
	if read, err := d.reader.Read(buf); err != nil || uint32(read) < length {
		return "", fmt.Errorf("Cannot read %d byte from reader", length)
	}
	str := string(buf)
	if length > 0 {
		d.stringRefs.Add(str)
	}
	return str, nil
}

func checkError(v reflect.Value, err error) (reflect.Value, error) {
	if err != nil {
		return reflect.Zero(reflect.TypeOf(v.Interface())), err
	}
	return v, nil
}

func (d *Decoder) readReflectValue(v *reflect.Value) error {
	marker, err := d.readUInt8()
	if err != nil {
		return err
	}

	//fmt.Println("Marker:", marker)

	switch Marker(marker) {
	case MarkerUndefined:
		return setReflectValue(v, reflect.ValueOf(AMF3Undefined{}))
	case MarkerNull:
		return setReflectValue(v, reflect.ValueOf(AMF3Null{}))
	case MarkerFalse:
		return setReflectValue(v, reflect.ValueOf(false))
	case MarkerTrue:
		return setReflectValue(v, reflect.ValueOf(true))
	case MarkerInteger:
		i, err := d.readUInt29()
		if err != nil {
			return err
		}
		// TODO sign extension
		return setReflectValue(v, reflect.ValueOf(i))
	case MarkerDouble:
		var f float64
		if err := binary.Read(d.reader, binary.BigEndian, &f); err != nil {
			return err
		}
		return setReflectValue(v, reflect.ValueOf(f))
	case MarkerString:
		str, err := d.readString()
		if err != nil {
			return err
		}
		return setReflectValue(v, reflect.ValueOf(str))
	case MarkerXMLDoc:
		// TODO
	case MarkerXML:
		// TODO
	case MarkerByteArray:
		// TODO
	case MarkerDate:
		// TODO
	case MarkerArray:
		if !v.IsValid() {
			*v = reflect.ValueOf(new(TypedObject))
		}
		if err := d.readArray(v); err != nil {
			return err
		}
		return nil
	case MarkerObject:
		ref, err := d.readUInt29()
		if err != nil {
			return err
		}
		//fmt.Println("ref", ref)
		if (ref & 1) == 0 {
			if val, err := d.objectRefs.Get(int(ref >> 1)); err != nil {
				return err
			} else {
				return setReflectValue(v, reflect.ValueOf(val))
			}
		}
		var traits *Traits
		if (ref & 3) == 1 {
			if t, err := d.traitsRefs.Get(int(ref >> 2)); err != nil {
				return err
			} else {
				traits = t.(*Traits)
				//fmt.Println("Traits ref: class="+traits.ClassName, "members=", traits.Members)
			}
		} else {
			traits = &Traits{}

			if (ref & 7) == 7 {
				traits.External = true
			} else {
				traits.Dynamic = (ref>>3)&1 == 1
				traits.Nmemb = int(ref >> 4)
			}

			cls, err := d.readString()
			if err != nil {
				return err
			}
			traits.ClassName = cls

			for i := 0; i < traits.Nmemb; i++ {
				key, err := d.readString()
				if err != nil {
					return err
				}
				traits.Members = append(traits.Members, key)
			}

			d.traitsRefs.Add(traits)
		}

		if traits.External {
			at := len(d.objectRefs)
			d.objectRefs = append(d.objectRefs, nil)
			if err := d.readExternalObject(traits, v); err != nil {
				return err
			}
			d.objectRefs[at] = v
			return nil
		} else {
			obj := *v
			if !v.IsValid() {
				obj = d.createReflectObject(traits)
			}
			d.objectRefs.Add(obj.Interface())
			switch tobj := obj.Interface().(type) {
			case *TypedObject:
				for _, key := range traits.Members {
					var val reflect.Value
					err := d.readReflectValue(&val)
					if err != nil {
						return err
					}
					//fmt.Println("Read key", key, "value", val.Interface())
					tobj.Assoc[key] = reflect.Indirect(val).Interface()
				}
			default:
				for _, key := range traits.Members {
					lowerCase := strings.ToLower(key)
					field := reflect.Indirect(obj).FieldByNameFunc(func(name string) bool {
						return strings.ToLower(name) == lowerCase
					})

					y := false
					if !field.IsValid() {
						field = reflect.Value{}
						//fmt.Println("Skipped field not found in struct:", key)
					} else {
						y = true
					}

					if err := d.readReflectValue(&field); err != nil {
						return err
					}

					if y {
						//fmt.Printf("key %s [valid=%t] in class %s: %s\n", key, field.IsValid(), traits.ClassName, field)
					}

					/*
						if err := simpleSet(field, reflect.Indirect(val).Interface()); err != nil {
							return fmt.Errorf("key %s in class %s: %s", key, traits.ClassName, err)
						}
					*/
				}
			}

			if traits.Dynamic {
				for {
					key, err := d.readString()
					if err != nil {
						return err
					}
					if len(key) == 0 {
						break
					}
					var val reflect.Value
					if err := d.readReflectValue(&val); err != nil {
						return err
					} else if tobj, ok := obj.Interface().(*TypedObject); ok {
						tobj.Assoc[key] = reflect.Indirect(val).Interface()
					} else {
						//fmt.Println("Skipped dynamic field:", key)
					}
				}
			}
			return setReflectValue(v, obj)
		}
		panic("Not reached")
	}

	return fmt.Errorf("Unhandled marker: %d", marker)
}

func (d *Decoder) readArray(v *reflect.Value) error {
	length, err := d.readUInt29()
	if err != nil {
		return err
	}
	if (length & 1) == 0 {
		if val, err := d.objectRefs.Get(int(length >> 1)); err != nil {
			return err
		} else {
			v.Set(reflect.ValueOf(val))
			return nil
		}
	}
	length >>= 1

	d.objectRefs.Add(v.Interface())

	tobj, isTypedObject := v.Interface().(*TypedObject)
	assocCappable, arrayCappable := false, false
	switch v.Kind() {
	case reflect.Map:
		assocCappable = true
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
	case reflect.Slice, reflect.Array:
		arrayCappable = true
		if v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
	}

	for {
		key, err := d.readString()
		if err != nil {
			return err
		}
		if len(key) == 0 {
			break
		}
		var val reflect.Value
		if err := d.readReflectValue(&val); err != nil {
			return err
		}
		if isTypedObject {
			tobj.Assoc[key] = reflect.Indirect(val).Interface()
		} else if assocCappable {
			v.SetMapIndex(reflect.ValueOf(key), val)
		}
	}
	for i := 0; i < int(length); i++ {
		var val reflect.Value
		if err := d.readReflectValue(&val); err != nil {
			return err
		}
		if isTypedObject {
			tobj.Array = append(tobj.Array, val.Interface())
		} else if arrayCappable {
			appendReflect(v, val)
		}
	}
	return nil
}

func (d *Decoder) readExternalObject(traits *Traits, v *reflect.Value) error {
	// TODO
	switch traits.ClassName {
	case "flex.messaging.io.ArrayCollection":
		return d.readReflectValue(v)
	}
	return fmt.Errorf("External object not implemented: class=%s", traits.ClassName)
}

func (d *Decoder) createReflectObject(traits *Traits) reflect.Value {
	// TODO
	//fmt.Println(traits)
	if dt, ok := userdefinedTypes[traits.ClassName]; ok {
		return reflect.New(dt.Type)
	}
	return reflect.ValueOf(&TypedObject{
		Assoc: make(map[string]interface{}),
		Array: make([]interface{}, 0, 0),
	})
}
