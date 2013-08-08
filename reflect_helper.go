package amf

import (
	"errors"
	"fmt"
	"reflect"
)

func assignOrSetReflectValue(dst reflect.Value, src reflect.Value) error {
	if !dst.CanSet() {
		return errors.New("dst is not settable")
	}
	if !src.Type().AssignableTo(dst.Type()) {
		if src.Type().ConvertibleTo(dst.Type()) {
			src = src.Convert(dst.Type())
		} else if _, ok := src.Interface().(AMF3Null); ok {
			dst.Set(reflect.New(dst.Type()).Elem())
			return nil
		} else {
			return fmt.Errorf("Cannot set type %s to %s", src.Kind(), dst.Kind())
		}
	}
	dst.Set(src)
	return nil
}

func setReflectValue(v *reflect.Value, newv reflect.Value) error {
	if v.IsValid() {
		switch v.Kind() {
		case reflect.Ptr:
			if newv.Kind() != reflect.Ptr && newv.CanAddr() {
				newv = newv.Addr()
			}
		default:
			if newv.Kind() == reflect.Ptr {
				newv = newv.Elem()
			}
		}
		assignOrSetReflectValue(*v, newv)
	} else {
		*v = newv
	}
	return nil
}

func appendReflect(v *reflect.Value, elem reflect.Value) error {
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("type %s cannot be appended to", v.Kind())
	}
	switch v.Type().Elem().Kind() {
	case reflect.Ptr:
		if elem.Kind() == reflect.Ptr {
			v.Set(reflect.Append(*v, elem))
		} else if elem.CanAddr() {
			v.Set(reflect.Append(*v, elem.Addr()))
		}
	default:
		if elem.Kind() == reflect.Ptr {
			v.Set(reflect.Append(*v, elem.Elem()))
		}
	}
	return nil
}
