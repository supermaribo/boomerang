package api

import (
	"database/sql"
	"reflect"
)

// normalizeNilSlices ensures JSON encodes empty lists as [] instead of null.
func normalizeNilSlices(v any) any {
	if v == nil {
		return nil
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Slice:
		if val.IsNil() {
			return reflect.MakeSlice(val.Type(), 0, 0).Interface()
		}
		return v
	case reflect.Map:
		if val.Type().Key().Kind() != reflect.String {
			return v
		}
		out := make(map[string]any, val.Len())
		for _, key := range val.MapKeys() {
			out[key.String()] = normalizeNilSlices(val.MapIndex(key).Interface())
		}
		return out
	case reflect.Ptr:
		if val.IsNil() {
			// Return the typed nil pointer, not an untyped nil, so callers
			// that assign back into struct fields get a valid reflect.Value.
			return v
		}
		inner := normalizeNilSlices(val.Elem().Interface())
		rv := reflect.ValueOf(inner)
		if rv.IsValid() && rv.Type().AssignableTo(val.Type().Elem()) {
			p := reflect.New(val.Type().Elem())
			p.Elem().Set(rv)
			return p.Interface()
		}
		return v
	case reflect.Interface:
		if val.IsNil() {
			return nil
		}
		return normalizeNilSlices(val.Elem().Interface())
	case reflect.Struct:
		if val.Type() == reflect.TypeOf(sql.NullString{}) {
			return v
		}
		out := reflect.New(val.Type()).Elem()
		out.Set(val)
		for i := 0; i < out.NumField(); i++ {
			f := out.Field(i)
			if !f.CanSet() {
				continue
			}
			nv := normalizeNilSlices(f.Interface())
			if nv == nil {
				continue
			}
			rv := reflect.ValueOf(nv)
			if rv.Type().AssignableTo(f.Type()) {
				f.Set(rv)
			}
		}
		return out.Interface()
	default:
		return v
	}
}
