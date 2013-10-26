package amf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type ExternalizeWritable interface {
	WriteExternal(e *Encoder) error
}

type Encoder struct {
	VerboseLog bool

	TraitsMapper *TraitsMapper

	writer io.Writer

	stringRefs refTable
	objectRefs refTable
	traitsRefs refTable

	stringRefMap  map[string]int
	traitsRefsMap map[string]int
}

func NewEncoder(writer io.Writer) *Encoder {
	return &Encoder{
		writer: writer,

		stringRefMap:  make(map[string]int),
		traitsRefsMap: make(map[string]int),
	}
}

func (e *Encoder) Encode(obj interface{}) error {
	return e.WriteValue(obj)
}

func (e *Encoder) WriteUInt8(v uint8) error {
	_, err := e.writer.Write([]byte{byte(v)})
	return err
}

func (e *Encoder) WriteUInt29(v uint32) error {
	const mask uint32 = 0x3FFFFFFF

	if (v & ^mask) > 0 {
		return errors.New("writeUInt29: value too big")
	}

	b := make([]byte, 4, 4)
	if (v >> 21) > 0 {
		b[0] = 0x80 | byte((v>>22)&0x7F)
		b[1] = 0x80 | byte((v>>15)&0x7F)
		b[2] = 0x80 | byte((v>>8)&0x7F)
		b[3] = byte(v & 0x7F)
	} else {
		b = b[:3]
		b[0] = byte((v >> 14) & 0x7F)
		b[1] = byte((v >> 7) & 0x7F)
		b[2] = byte(v & 0x7F)
		for ; len(b) > 1 && b[0] == 0; b = b[1:] {
		}
		for i := 0; i < len(b)-1; i++ {
			b[i] |= 0x80
		}
	}
	_, err := e.writer.Write(b)
	return err
}

func (e *Encoder) WriteString(str string) error {
	// Only non-empty strings are put into ref table
	if len(str) > 0 {
		if ref, ok := e.stringRefMap[str]; ok {
			e.logPrintln("WriteString using ref", ref)
			return e.WriteUInt29(uint32(ref << 1))
		}

		e.logPrintln("Add string to ref table", "ref", e.stringRefs.Len(), str)
		e.stringRefMap[str] = e.stringRefs.Len()
		e.stringRefs.Add(str)
	}

	e.logPrintln("Write string to buffer", "len", len(str), str)
	if err := e.WriteUInt29(uint32(len(str)<<1) | 0x1); err != nil {
		return err
	}

	_, err := e.writer.Write([]byte(str))
	return err
}

func (e *Encoder) writeMarker(marker Marker) error {
	return e.WriteUInt8(uint8(marker))
}

func (e *Encoder) WriteValue(vif interface{}) error {
	v := reflect.ValueOf(vif)
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		v = v.Elem()
	}
	if !v.IsValid() {
		return e.writeMarker(MarkerNull)
	}

	const maxUInt29 int64 = 0x3FFFFFFF
	const maxInt29 int64 = 0x1FFFFFFF
	const minInt29 int64 = -0x1FFFFFFF - 1

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return e.writeMarker(MarkerTrue)
		} else {
			return e.writeMarker(MarkerFalse)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := v.Int()
		if i < minInt29 || i > maxInt29 {
			return e.WriteValue(float64(i))
		}
		if err := e.writeMarker(MarkerInteger); err != nil {
			return err
		}
		return e.WriteUInt29(uint32(i & maxUInt29))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i := v.Uint()
		if i > uint64(maxInt29) {
			return e.WriteValue(float64(i))
		}
		if err := e.writeMarker(MarkerInteger); err != nil {
			return err
		}
		return e.WriteUInt29(uint32(i))

	case reflect.Float32, reflect.Float64:
		if err := e.writeMarker(MarkerDouble); err != nil {
			return err
		}
		return binary.Write(e.writer, binary.BigEndian, v.Float())

	case reflect.String:
		if err := e.writeMarker(MarkerString); err != nil {
			return err
		}
		return e.WriteString(v.String())

	case reflect.Slice:
		if err := e.writeMarker(MarkerArray); err != nil {
			return err
		}
		return e.WriteArray(vif)

	case reflect.Map:
		panic("TODO") // TODO

	case reflect.Struct:
		if err := e.writeMarker(MarkerObject); err != nil {
			return err
		}
		return e.WriteObject(vif)

	default:
		return fmt.Errorf("Unhandled kind: %v", v.Kind())
	}
	panic("Not reached")
}

func (e *Encoder) WriteArray(vif interface{}) error {
	v := reflect.ValueOf(vif)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		panic("Must be a slice")
	}

	e.objectRefs.Add(v.Interface())
	sl := v.Interface().([]interface{})

	// Length and ref
	if err := e.WriteUInt29(uint32(len(sl)<<1) | 0x1); err != nil {
		return err
	}

	// Member is not supported.
	if err := e.WriteString(""); err != nil {
		return err
	}

	// Dense part
	for _, el := range sl {
		e.logPrintln("ARRAY write element", el)
		if err := e.WriteValue(el); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) WriteObject(vif interface{}) error {
	v := reflect.ValueOf(vif)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Must be a struct")
	}

	traits, ref := e.getReflectTraits(v)
	if traits == nil {
		// TODO
		e.logPrintln("No traits for type", v.Type())
		panic("TODO")
	}
	if ref >= 0 {
		if err := e.WriteUInt29(uint32(ref<<2) | 0x1); err != nil {
			return err
		}
	} else {
		if err := e.writeTraits(traits); err != nil {
			return err
		}
	}

	if v.CanAddr() {
		e.objectRefs.Add(v.Addr().Interface())
	} else {
		e.objectRefs.Add(v.Interface())
	}

	// Handle external object
	if traits.External {
		if encobj, ok := v.Addr().Interface().(ExternalizeWritable); ok {
			return encobj.WriteExternal(e)
		} else {
			return errors.New("Object is not externalizable")
		}
	}

	switch v.Interface().(type) {
	case TypedObject:
		// TODO
		panic("TODO")

	default:
		for _, key := range traits.Members {
			field := findFieldByName(v, key)

			var fif interface{} = nil
			if field.IsValid() {
				fif = field.Interface()
			}

			if err := e.WriteValue(fif); err != nil {
				return err
			}
		}
		if traits.Dynamic {
			if v.IsValid() {
				stype := v.Type()
				for i := 0; i < stype.NumField(); i++ {
					sf := stype.Field(i)

					// Only exported fields
					if sf.PkgPath != "" {
						continue
					}

					if sf.Tag.Get("amf3_dynamic") == "" {
						continue
					}

					field := v.Field(i)
					if !field.IsValid() {
						continue
					}

					if err := e.WriteString(sf.Name); err != nil {
						return err
					}
					if err := e.WriteValue(field.Interface()); err != nil {
						return err
					}
				}
			}
			// End of dynamic fields
			if err := e.WriteString(""); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Encoder) writeTraits(traits *Traits) error {
	if len(traits.ClassName) > 0 {
		e.traitsRefsMap[traits.ClassName] = e.traitsRefs.Len()
		e.traitsRefs.Add(traits)
	}

	ref := uint32(3)

	if traits.External {
		ref |= 0x4
	} else {
		if traits.Dynamic {
			ref |= 0x8
		}
		ref |= uint32(traits.Nmemb) << 4
	}

	if err := e.WriteUInt29(ref); err != nil {
		return err
	}
	if err := e.WriteString(traits.ClassName); err != nil {
		return err
	}

	if !traits.External {
		for i := 0; i < traits.Nmemb; i++ {
			if err := e.WriteString(traits.Members[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Encoder) getReflectTraits(v reflect.Value) (traits *Traits, ref int) {
	mapper := DefaultTraitsMapper
	if e.TraitsMapper != nil {
		mapper = e.TraitsMapper
	}
	dt := mapper.FindByReflectType(v.Type())

	if dt == nil {
		return nil, -1
	} else if ref, ok := e.traitsRefsMap[dt.Traits.ClassName]; ok {
		return dt.Traits, ref
	} else {
		return dt.Traits, -1
	}
}

func (e *Encoder) logPrintln(objs ...interface{}) {
	if e.VerboseLog {
		fmt.Println(objs...)
	}
}
