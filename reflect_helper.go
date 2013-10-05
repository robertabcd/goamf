package amf

import (
	"fmt"
	"reflect"
)

func setReflectValue(dst reflect.Value, srcif interface{}) error {
	src := reflect.ValueOf(srcif)
	if !src.Type().AssignableTo(dst.Type()) {
		if src.Type().ConvertibleTo(dst.Type()) {
			src = src.Convert(dst.Type())
		} else {
			return fmt.Errorf("Cannot set type %s to %s", src.Kind(), dst.Kind())
		}
	}
	dst.Set(src)
	return nil
}

func createReflectObject(v reflect.Value, defaultType reflect.Type) error {
	//fmt.Println("createReflectObject", "v.Type()", v.Type(), "defaultType", defaultType)
	if !v.CanSet() {
		panic("v is not settable")
		//return fmt.Errorf("v is not settable")
	}
	switch v.Kind() {
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		return nil
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		return nil
	case reflect.Ptr:
		v.Set(reflect.New(v.Type().Elem()))
		return nil
	case reflect.Interface:
		v.Set(reflect.New(reflectRemoveTypePtrs(defaultType)))
		return nil
	default:
		return setReflectValue(v, reflect.New(defaultType).Elem().Interface())
	}
}

func reflectRemoveTypePtrs(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func reflectResolveType(dstKind reflect.Kind, src reflect.Value) reflect.Value {
	switch {
	case dstKind == reflect.Ptr && src.Kind() != reflect.Ptr:
		return src.Addr()
	case dstKind != reflect.Ptr && src.Kind() == reflect.Ptr:
		return src.Elem()
	default:
		return src
	}
}

func assignOrSetReflectValue(dst reflect.Value, src reflect.Value) error {
	switch dst.Kind() {
	case reflect.Ptr, reflect.Interface:
		switch src.Kind() {
		case reflect.Ptr:
			return directAssignOrSetReflectValue(dst, src)
		default:
			if src.CanAddr() {
				return directAssignOrSetReflectValue(dst, src.Addr())
			} else {
				return directAssignOrSetReflectValue(dst, src)
			}
		}
	default:
		switch src.Kind() {
		case reflect.Ptr:
			return directAssignOrSetReflectValue(dst, src.Elem())
		default:
			return directAssignOrSetReflectValue(dst, src)
		}
	}
}

func directAssignOrSetReflectValue(dst reflect.Value, src reflect.Value) error {
	if !dst.CanSet() {
		panic("dst is not settable")
		//return errors.New("dst is not settable")
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

func assignOrSetReflectAnyPtr(dst reflect.Value, srcptr interface{}) error {
	return assignOrSetReflectValue(dst, reflect.ValueOf(srcptr))
}

func resolveReflectPtrs(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if !v.Elem().IsValid() {
			return v
		}
		v = v.Elem()
	}
	return v
}

func resolveReflectPtrAndInterfaces(v reflect.Value) reflect.Value {
	for {
		switch v.Kind() {
		case reflect.Ptr:
			if !v.Elem().IsValid() {
				return v
			}
		case reflect.Interface:
			if v.IsNil() {
				return v
			}
		default:
			return v
		}
		v = v.Elem()
	}
}

//func setReflectValue(v reflect.Value, newv reflect.Value) error {
//	if v.IsValid() {
//		switch v.Kind() {
//		case reflect.Ptr:
//			if newv.Kind() != reflect.Ptr && newv.CanAddr() {
//				newv = newv.Addr()
//			}
//		default:
//			if newv.Kind() == reflect.Ptr {
//				newv = newv.Elem()
//			}
//		}
//		assignOrSetReflectValue(v, newv)
//	} else {
//		v.Set(newv)
//	}
//	return nil
//}

// v must be a settable slice or interface.
// if v is a interface, underlying type must be a slice.
// if v is []*T, elem must be either *T or addressable value of T.
// if v is []T, elem must be either *T or T.
func appendReflect(v reflect.Value, elem reflect.Value) error {
	if !v.CanSet() {
		return fmt.Errorf("v(Type %v) is not settable", v.Type())
	}
	sv := v
	if sv.Kind() == reflect.Interface {
		sv = v.Elem()
	}
	if sv.Kind() != reflect.Slice {
		return fmt.Errorf("type %s cannot be appended to", v.Kind())
	}
	switch sv.Type().Elem().Kind() {
	case reflect.Ptr:
		if elem.Kind() == reflect.Ptr {
			v.Set(reflect.Append(sv, elem))
		} else if elem.CanAddr() {
			v.Set(reflect.Append(sv, elem.Addr()))
		} else {
			return fmt.Errorf("elem is not a addressable value")
		}
	default:
		if elem.Kind() == reflect.Ptr {
			v.Set(reflect.Append(sv, elem.Elem()))
		} else {
			v.Set(reflect.Append(sv, elem))
		}
	}
	return nil
}
