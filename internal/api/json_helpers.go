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
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return nil
		}
		return normalizeNilSlices(val.Elem().Interface())
	case reflect.Struct:
		if val.Type() == reflect.TypeOf(sql.NullString{}) {
			return v
		}
		t := val.Type()
		out := reflect.New(t).Elem()
		for i := 0; i < val.NumField(); i++ {
			if !out.Field(i).CanSet() {
				continue
			}
			out.Field(i).Set(reflect.ValueOf(normalizeNilSlices(val.Field(i).Interface())))
		}
		return out.Interface()
	default:
		return v
	}
}
