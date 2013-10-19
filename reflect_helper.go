package amf

import (
	"fmt"
	"reflect"
	"strings"
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

func findFieldByName(v reflect.Value, name string) reflect.Value {
	name = strings.ToLower(name)
	tp := v.Type()
	for i, n := 0, tp.NumField(); i < n; i++ {
		sf := tp.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if strings.ToLower(sf.Name) == name || sf.Tag.Get("amf3") == name {
			return v.Field(i)
		}
		if sf.Anonymous {
			if field := findFieldByName(v.Field(i), name); field.IsValid() {
				return field
			}
		}
	}
	return reflect.ValueOf(nil)
}

func getStructMembersAndDynamics(v reflect.Value) (members []string, dynamics []string) {
	return appendStructMembersAndDynamics(v, nil, nil)
}

func appendStructMembersAndDynamics(v reflect.Value, members []string, dynamics []string) ([]string, []string) {
	tp := v.Type()
	for i, n := 0, tp.NumField(); i < n; i++ {
		sf := tp.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if sf.Anonymous {
			members, dynamics = appendStructMembersAndDynamics(v.Field(i), members, dynamics)
		} else {
			name := sf.Tag.Get("amf3")
			if name == "" {
				name = sf.Name
			}
			if sf.Tag.Get("amf3_dynamic") == "" {
				members = append(members, name)
			} else {
				dynamics = append(dynamics, name)
			}
		}
	}
	return members, dynamics
}
