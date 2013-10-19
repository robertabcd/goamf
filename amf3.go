package amf

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"time"
)

type Decoder struct {
	TraitsMapper *TraitsMapper

	reader io.Reader

	stringRefs refTable
	objectRefs refTable
	traitsRefs refTable
}

type ExternalizeReadable interface {
	ReadExternal(d *Decoder) error
}

func NewDecoder(reader io.Reader) *Decoder {
	return &Decoder{
		reader: reader,
	}
}

func (d *Decoder) Decode(objptr interface{}) error {
	return d.ReadValue(objptr)
}

func (d *Decoder) GetReader() io.Reader {
	return d.reader
}

func (d *Decoder) ReadUInt8() (uint8, error) {
	b := []byte{0}
	_, err := d.reader.Read(b)
	return b[0], err
}

func (d *Decoder) ReadUInt29() (uint32, error) {
	var v uint32
	for j := 0; j < 3; j++ {
		b, err := d.ReadUInt8()
		if err != nil {
			return 0, err
		}
		v = (v << 7) | (uint32(b) & 0x7F)
		if (b & 0x80) != 0x80 {
			return v, nil
		}
	}
	b, err := d.ReadUInt8()
	if err != nil {
		return 0, err
	}
	v = (v << 8) | uint32(b)
	return v, err
}

func (d *Decoder) ReadString() (string, error) {
	length, err := d.ReadUInt29()
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

func (d *Decoder) ReadValue(vptr interface{}) error {
	marker, err := d.ReadUInt8()
	if err != nil {
		return err
	}

	switch Marker(marker) {
	case MarkerArray:
		return d.ReadArray(vptr)
	case MarkerObject:
		return d.ReadObject(vptr)
	}

	v := reflect.ValueOf(vptr)
	if v.Kind() != reflect.Ptr {
		panic("Must pass a pointer")
	}
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	v = v.Elem()

	switch Marker(marker) {
	case MarkerUndefined:
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			return setReflectValue(v, &AMF3Undefined{})
		default:
			return fmt.Errorf("Read Undefined: Incompatible type")
		}
	case MarkerNull:
		v.Set(reflect.New(v.Type()).Elem())
		return nil
	case MarkerFalse:
		return setReflectValue(v, false)
	case MarkerTrue:
		return setReflectValue(v, true)
	case MarkerInteger:
		i, err := d.ReadUInt29()
		if err != nil {
			return err
		}
		// TODO sign extension
		return setReflectValue(v, i)
	case MarkerDouble:
		var f float64
		if err := binary.Read(d.reader, binary.BigEndian, &f); err != nil {
			return err
		}
		return setReflectValue(v, f)
	case MarkerString:
		str, err := d.ReadString()
		if err != nil {
			return err
		}
		return setReflectValue(v, str)
	case MarkerDate:
		ref, err := d.ReadUInt29()
		if err != nil {
			return err
		}
		if ref&1 == 0 {
			if val, err := d.objectRefs.Get(int(ref >> 1)); err != nil {
				return err
			} else {
				return setReflectValue(v, val)
			}
		} else {
			var f float64
			if err := binary.Read(d.reader, binary.BigEndian, &f); err != nil {
				return err
			}
			t := time.Unix(int64(f), 0)
			d.objectRefs.Add(t)
			return setReflectValue(v, t)
		}
	case MarkerXMLDoc:
		// TODO
	case MarkerXML:
		// TODO
	case MarkerByteArray:
		// TODO
	}

	return fmt.Errorf("Unhandled marker: %d", marker)
}

func (d *Decoder) ReadArray(vptr interface{}) error {
	v := reflect.ValueOf(vptr)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("vptr must be a pointer")
	}
	v = v.Elem()
	if !v.CanSet() {
		panic("ReadObject: v must be settable")
	}

	length, err := d.ReadUInt29()
	if err != nil {
		return err
	}
	if (length & 1) == 0 {
		if val, err := d.objectRefs.Get(int(length >> 1)); err != nil {
			return err
		} else {
			return setReflectValue(v, val)
		}
	}
	length >>= 1

	if v.IsNil() {
		if err := createReflectObject(v, reflect.TypeOf(TypedObject{})); err != nil {
			return err
		}
	}

	d.objectRefs.Add(v.Interface())

	tobj, isTypedObject := v.Interface().(*TypedObject)
	assocCappable, arrayCappable := false, false
	switch v.Kind() {
	case reflect.Map:
		assocCappable = true
	case reflect.Slice:
		arrayCappable = true
	}
	d.logPrintln("ReadArray to", v.Type(), "isTypedObject", isTypedObject, "assocCappable", assocCappable, "arrayCappable", arrayCappable)

readMemberLoop:
	for {
		key, err := d.ReadString()
		switch {
		case err != nil:
			return err
		case len(key) == 0:
			break readMemberLoop
		}

		if isTypedObject {
			var val interface{}
			if err := d.ReadValue(&val); err != nil {
				return err
			}
			tobj.Assoc[key] = val
		} else {
			var val interface{}
			if err := createReflectObject(reflect.ValueOf(&val).Elem(), v.Type().Elem()); err != nil {
				return err
			}
			vrefl := reflect.ValueOf(val)
			if vrefl.Kind() != reflect.Ptr {
				vrefl = vrefl.Addr()
			}
			if err := d.ReadValue(vrefl.Interface()); err != nil {
				return err
			}
			if assocCappable {
				v.SetMapIndex(reflect.ValueOf(key), reflectResolveType(v.Type().Elem().Kind(), reflect.ValueOf(val)))
			} else {
				d.logPrintln("Ignore key", key)
			}
		}
	}
	for i := 0; i < int(length); i++ {
		if isTypedObject {
			var val interface{}
			if err := d.ReadValue(&val); err != nil {
				return err
			}
			tobj.Array = append(tobj.Array, val)
		} else {
			var val interface{}
			if err := createReflectObject(reflect.ValueOf(&val).Elem(), v.Type().Elem()); err != nil {
				return err
			}
			vrefl := reflect.ValueOf(val)
			d.logPrintln("vrefl", vrefl.Interface(), "Type", vrefl.Type())
			if vrefl.Kind() != reflect.Ptr {
				vrefl = vrefl.Addr()
			}
			if err := d.ReadValue(vrefl.Interface()); err != nil {
				return err
			}
			if arrayCappable {
				v.Set(reflect.Append(v, reflectResolveType(v.Type().Elem().Kind(), reflect.ValueOf(val))))
			} else {
				d.logPrintln("Ignore index", i)
			}
		}
	}
	return nil
}

func (d *Decoder) ReadObject(vptr interface{}) error {
	v := reflect.ValueOf(vptr)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("vptr must be a pointer")
	}
	v = v.Elem()

	ref, err := d.ReadUInt29()
	if err != nil {
		return err
	}

	// Handle reference to object
	if (ref & 1) == 0 {
		if val, err := d.objectRefs.Get(int(ref >> 1)); err != nil {
			return err
		} else {
			return setReflectValue(v, val)
		}
	}

	// Read object traits or find from reference
	var traits *Traits
	if (ref & 3) == 1 {
		if t, err := d.traitsRefs.Get(int(ref >> 2)); err != nil {
			return err
		} else {
			traits = t.(*Traits)
			d.logPrintln("Traits ref: class="+traits.ClassName, "members=", traits.Members)
		}
	} else {
		traits = &Traits{}

		if (ref & 7) == 7 {
			traits.External = true
		} else {
			traits.Dynamic = (ref>>3)&1 == 1
			traits.Nmemb = int(ref >> 4)
		}

		cls, err := d.ReadString()
		if err != nil {
			return err
		}
		traits.ClassName = cls

		for i := 0; i < traits.Nmemb; i++ {
			key, err := d.ReadString()
			if err != nil {
				return err
			}
			traits.Members = append(traits.Members, key)
		}

		d.traitsRefs.Add(traits)
	}

	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			if err := setReflectValue(v, d.createObject(traits.ClassName)); err != nil {
				return err
			}
			if v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}
			d.logPrintln("Initialized value for object", traits.ClassName, "Type", v.Type())
		}
	default:
		v.Set(reflect.New(v.Type()).Elem())
		d.logPrintln("Set zero value for object", traits.ClassName, "Type", v.Type())
	}
	if v.Kind() == reflect.Ptr {
		panic("Must be resolved")
	}

	d.objectRefs.Add(v.Interface())

	d.logPrintln("Object Traits", traits)
	// Handle external object
	if traits.External {
		refobj := d.objectRefs.Reserve()
		if err := d.readExternalObject(traits, vptr); err != nil {
			return err
		}
		*refobj = v
		return nil
	}

	switch tobj := v.Interface().(type) {
	case TypedObject:
		d.logPrintln("ReadObject: reading into TypedObject")
		for _, key := range traits.Members {
			var val interface{}
			if err := d.ReadValue(&val); err != nil {
				return err
			}
			d.logPrintln("Read key", key, "value", val)
			tobj.Assoc[key] = val
		}
		if traits.Dynamic {
		readDyanmicLoopTO:
			for {
				var val interface{}
				key, err := d.ReadString()
				switch {
				case err != nil:
					return err
				case len(key) == 0:
					break readDyanmicLoopTO
				default:
					if err := d.ReadValue(&val); err != nil {
						return err
					}
					d.logPrintln("Read dynamic key", key, "value", val)
					tobj.Assoc[key] = val
				}
			}
		}
		d.logPrintln("Read TypedObject:", tobj)
	default:
		for _, key := range traits.Members {
			if _, err := d.readObjectField(v, key); err != nil {
				return err
			}
		}
		if traits.Dynamic {
		readDyanmicLoop:
			for {
				key, err := d.ReadString()
				switch {
				case err != nil:
					return err
				case len(key) == 0:
					break readDyanmicLoop
				default:
					if ignored, err := d.readObjectField(v, key); err != nil {
						return err
					} else if ignored {
						d.logPrintln("ignore key", key, "in class", traits.ClassName)
					}
				}
			}
		}
	}

	return nil
}

func (d *Decoder) readObjectField(v reflect.Value, key string) (ignored bool, err error) {
	if !v.CanSet() {
		panic("readObjectField: v must be settable")
	}

	var field reflect.Value

	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		elemType := v.Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			field = reflect.New(elemType.Elem())
		} else {
			field = reflect.New(elemType)
		}

	default:
		field = findFieldByName(v, key)

		if !field.IsValid() {
			var tmpobj interface{}
			field = reflect.ValueOf(&tmpobj).Elem()
			d.logPrintln("Prepared dummy value for non-exist key", key)
			ignored = true
		}
		if !field.CanSet() {
			d.logPrintln("v.Type()", v.Type())
			d.logPrintln("field.Type():", field.Type())
			panic("field not settable")
		}
		d.logPrintln("Original field type", field.Type())
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				if err = createReflectObject(field, field.Type().Elem()); err != nil {
					return
				}
			}
		} else {
			field = field.Addr()
		}
	}

	err = d.ReadValue(field.Interface())
	d.logPrintln("Read field key", key, "type", field.Type(), "value", field.Elem().Interface())

	if v.Kind() == reflect.Map {
		if v.Type().Elem().Kind() != reflect.Ptr {
			field = field.Elem()
		}
		v.SetMapIndex(reflect.ValueOf(key), field)
	}

	return
}

func (d *Decoder) readExternalObject(traits *Traits, vptr interface{}) error {
	switch traits.ClassName {
	case "flex.messaging.io.ArrayCollection":
		return d.ReadValue(vptr)
	}
	obj := d.createObject(traits.ClassName)
	if extobj, ok := obj.(ExternalizeReadable); ok {
		return extobj.ReadExternal(d)
	}
	return fmt.Errorf("External object not implemented: class=%s", traits.ClassName)
}

func (d *Decoder) createObject(className string) interface{} {
	mapper := DefaultTraitsMapper
	if d.TraitsMapper != nil {
		mapper = d.TraitsMapper
	}
	if dt := mapper.FindByClassName(className); dt != nil {
		return reflect.New(dt.Type).Interface()
	}
	return &TypedObject{
		Assoc: make(map[string]interface{}),
		Array: make([]interface{}, 0, 0),
	}
}

func (d *Decoder) logPrintln(objs ...interface{}) {
	//fmt.Println(objs...)
}
