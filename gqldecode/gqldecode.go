// Package gqldecode provides a decoder that unmarshals GraphQL input arguments into a struct.
// The struct for the input arguments is annotated with tags similar to the stdlib json parser,
// but instead of "json" the key "gql" is used. The options "nonzero" and "plane0" can be used
// to signify that the field should not be the zero value and that a string field must be plan0 utf8
// (i.e. no emoji).
package gqldecode

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const tagName = "gql"

// ValidationFailedError is returned if the input doesn't match the expected output format.
type ValidationFailedError struct {
	Field  string
	Reason string
}

func (e *ValidationFailedError) Error() string {
	return fmt.Sprintf("gqldecode: field %s failed validation: %s", e.Field, e.Reason)
}

// Decoder is the interface implemented by types that know how to decode themselves.
type Decoder interface {
	DecodeGQL(interface{}) error
}

// Decode parses a map of strings to interfaces, as provided by the graphql library,
// into the provided out interface.
func Decode(in map[string]interface{}, out interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()
	outV := reflect.ValueOf(out)
	if outV.Kind() != reflect.Ptr || outV.Type().Elem().Kind() != reflect.Struct {
		return fmt.Errorf("gqldecode: Decode requires a pointer to a struct")
	}
	decodeStruct(in, outV.Elem())
	return nil
}

func decodeStruct(in map[string]interface{}, out reflect.Value) {
	si := infoForStruct(out.Type())
	for name, value := range in {
		fieldInfo := si.fields[name]
		if fieldInfo == nil {
			errf("gqldecode: field %s not found for struct %T", name, out)
		}
		field := out.Field(fieldInfo.index)
		decodeValue(value, field, fieldInfo)
	}
}

func decodeValue(v interface{}, out reflect.Value, fi *structFieldInfo) {
	if fi.hasDecoderMethod || fi.hasNonPtrDecoderMethod {
		if out.Kind() == reflect.Ptr && out.IsNil() {
			out.Set(reflect.New(out.Type().Elem()))
		}

		if fi.hasDecoderMethod {
			if err := out.Interface().(Decoder).DecodeGQL(v); err != nil {
				panic(err)
			}
		} else {
			if err := out.Addr().Interface().(Decoder).DecodeGQL(v); err != nil {
				panic(err)
			}
		}
		return
	}
	switch out.Kind() {
	case reflect.String:
		s, ok := v.(string)
		if !ok {
			// Possibly an alias string type (e.g. enum)
			s = reflect.ValueOf(v).String()
		}
		if fi.nonEmpty && s == "" {
			panic(&ValidationFailedError{Field: fi.name, Reason: "value may not be empty"})
		}
		if !utf8.ValidString(s) {
			panic(&ValidationFailedError{Field: fi.name, Reason: "value must be utf8 encoded"})
		}
		s = sanitizeUnicode(s)
		if fi.plane0Unicode && !IsValidPlane0Unicode(s) {
			panic(&ValidationFailedError{Field: fi.name, Reason: "value must be plane0 unicode"})
		}
		out.SetString(strings.TrimSpace(s))
	case reflect.Int, reflect.Int64:
		out.SetInt(int64(v.(int)))
	case reflect.Bool:
		out.SetBool(v.(bool))
	case reflect.Float64:
		out.SetFloat(v.(float64))
	case reflect.Slice:
		inS := v.([]interface{})
		outS := reflect.MakeSlice(out.Type(), len(inS), len(inS))
		for i, v := range inS {
			decodeValue(v, outS.Index(i), fi)
		}
		out.Set(outS)
	case reflect.Struct:
		_, isTime := out.Interface().(time.Time)
		_, isTimePtr := out.Interface().(*time.Time)
		if isTime || isTimePtr {
			var t time.Time
			switch v := v.(type) {
			case time.Time:
				t = v
			case *time.Time:
				t = *v
			case float64:
				t = time.Unix(int64(v), int64(1e9*(v-math.Floor(v)))).UTC()
			case int:
				t = time.Unix(int64(v), 0).UTC()
			case int64:
				t = time.Unix(v, 0).UTC()
			case string:
				var err error
				t, err = time.Parse(time.RFC3339Nano, v)
				if err != nil {
					errf("gqldecode: invalid datetime format for %q", v)
				}
			default:
				errf("gqldecode: invalid input type for time.Time %T", v)
			}
			if isTimePtr {
				out.Set(reflect.ValueOf(&t))
			} else {
				out.Set(reflect.ValueOf(t))
			}
			return
		}

		// in the event that the type is the same, or a pointer of the same type, set the value of
		// out to the value of v instead of assuming that v is a map[string]interface{} that can be
		// decoded into a struct.
		if out.Type() == reflect.TypeOf(v) {
			out.Set(reflect.ValueOf(v))
		} else if reflect.ValueOf(v).Kind() == reflect.Ptr && out.Type() == reflect.TypeOf(v).Elem() {
			out.Set(reflect.ValueOf(v).Elem())
		} else {
			decodeStruct(v.(map[string]interface{}), out)
		}
	case reflect.Ptr:
		if out.IsNil() {
			out.Set(reflect.New(out.Type().Elem()))
		}
		decodeValue(v, out.Elem(), fi)
	default:
		errf("gqldecode: unknown kind %s", out.Kind())
	}
}

func errf(msg string, v ...interface{}) {
	panic(fmt.Errorf("gqldecode: "+msg, v...))
}

type structFieldInfo struct {
	index                  int
	name                   string
	nonEmpty               bool
	plane0Unicode          bool
	hasDecoderMethod       bool
	hasNonPtrDecoderMethod bool
}

type structInfo struct {
	fields map[string]*structFieldInfo
}

var (
	structTypeCacheMu sync.RWMutex
	structTypeCache   = make(map[reflect.Type]*structInfo) // struct type -> field name -> field info
)

func infoForStruct(structType reflect.Type) *structInfo {
	structTypeCacheMu.RLock()
	sm := structTypeCache[structType]
	structTypeCacheMu.RUnlock()
	if sm != nil {
		return sm
	}

	structTypeCacheMu.Lock()
	defer structTypeCacheMu.Unlock()

	// Check again in case someone beat us
	sm = structTypeCache[structType]
	if sm != nil {
		return sm
	}

	sm = &structInfo{
		fields: make(map[string]*structFieldInfo),
	}
	decoderType := reflect.TypeOf((*Decoder)(nil)).Elem()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		tag := field.Tag
		tagValue := tag.Get(tagName)
		tagOptions := strings.Split(tagValue, ",")
		if len(tagOptions) != 0 {
			name := tagOptions[0]
			fi := &structFieldInfo{
				name:                   name,
				index:                  i,
				hasDecoderMethod:       field.Type.Implements(decoderType),
				hasNonPtrDecoderMethod: reflect.New(field.Type).Type().Implements(decoderType),
			}
			for _, opt := range tagOptions[1:] {
				switch opt {
				case "nonempty":
					fi.nonEmpty = true
				case "plane0":
					fi.plane0Unicode = true
				}
			}
			// Check for duplicate field names
			if _, ok := sm.fields[name]; ok {
				errf("duplicate field %s", name)
			}
			sm.fields[name] = fi
		}
	}
	structTypeCache[structType] = sm
	return sm
}
