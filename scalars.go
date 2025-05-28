package graphql

import (
	"fmt"
	"math"
	"strconv"

	"github.com/sprucehealth/graphql/language/ast"
)

func coerceInt(value any) any {
	switch v := value.(type) {
	case bool:
		if v {
			return 1
		}
		return 0
	case int:
		return value
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		if v < int64(math.MinInt32) || v > int64(math.MaxInt32) {
			return nil
		}
		return int(v)
	case uint:
		//nolint:gosec
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		if v > uint32(math.MaxInt32) {
			return nil
		}
		return int(v)
	case uint64:
		if v > uint64(math.MaxInt32) {
			return nil
		}
		return int(v)
	case float32:
		if v < float32(math.MinInt32) || v > float32(math.MaxInt32) {
			return nil
		}
		return int(v)
	case float64:
		if v < float64(math.MinInt64) || v > float64(math.MaxInt64) {
			return nil
		}
		return int(v)
	case string:
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return int(val)
	}

	// If the value cannot be transformed into an int, return nil instead of '0'
	// to denote 'no integer found'
	return nil
}

// Int is the GraphQL Integer type definition.
var Int = NewScalar(ScalarConfig{
	Name: "Int",
	Description: "The `Int` scalar type represents non-fractional signed whole numeric " +
		"values. Int can represent values between -(2^31) and 2^31 - 1. ",
	Serialize:  coerceInt,
	ParseValue: coerceInt,
	ParseLiteral: func(valueAST ast.Value) any {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			if intValue, err := strconv.Atoi(valueAST.Value); err == nil {
				return intValue
			}
		}
		return nil
	},
})

func coerceFloat64(value any) any {
	switch v := value.(type) {
	case bool:
		if v {
			return float64(1)
		}
		return float64(0)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case uint32:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return value
	case string:
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return val
	}
	return float64(0)
}

// Float is the GraphQL float type definition.
var Float = NewScalar(ScalarConfig{
	Name: "Float",
	Description: "The `Float` scalar type represents signed double-precision fractional " +
		"values as specified by " +
		"[IEEE 754](http://en.wikipedia.org/wiki/IEEE_floating_point). ",
	Serialize:  coerceFloat64,
	ParseValue: coerceFloat64,
	ParseLiteral: func(valueAST ast.Value) any {
		switch valueAST := valueAST.(type) {
		case *ast.FloatValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue
			}
		case *ast.IntValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue
			}
		}
		return nil
	},
})

func coerceString(value any) any {
	switch v := value.(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.Itoa(int(v))
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", value)
}

// String is the GraphQL string type definition
var String = NewScalar(ScalarConfig{
	Name: "String",
	Description: "The `String` scalar type represents textual data, represented as UTF-8 " +
		"character sequences. The String type is most often used by GraphQL to " +
		"represent free-form human-readable text.",
	Serialize:  coerceString,
	ParseValue: coerceString,
	ParseLiteral: func(valueAST ast.Value) any {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			return valueAST.Value
		}
		return nil
	},
})

func coerceBool(value any) any {
	switch v := value.(type) {
	case *bool:
		if v == nil {
			return false
		}
		return *v
	case bool:
		return value
	case string:
		switch v {
		case "", "false":
			return false
		}
		return true
	case float64:
		return v != 0.0
	case float32:
		return v != 0.0
	case int:
		return v != 0
	case int32:
		return v != 0
	case uint32:
		return v != 0
	case int64:
		return v != 0
	case uint64:
		return v != 0
	}
	return false
}

// Boolean is the GraphQL boolean type definition
var Boolean = NewScalar(ScalarConfig{
	Name:        "Boolean",
	Description: "The `Boolean` scalar type represents `true` or `false`.",
	Serialize:   coerceBool,
	ParseValue:  coerceBool,
	ParseLiteral: func(valueAST ast.Value) any {
		switch valueAST := valueAST.(type) {
		case *ast.BooleanValue:
			return valueAST.Value
		}
		return nil
	},
})

// ID is the GraphQL id type definition
var ID = NewScalar(ScalarConfig{
	Name: "ID",
	Description: "The `ID` scalar type represents a unique identifier, often used to " +
		"refetch an object or as key for a cache. The ID type appears in a JSON " +
		"response as a String; however, it is not intended to be human-readable. " +
		"When expected as an input type, any string (such as `\"4\"`) or integer " +
		"(such as `4`) input value will be accepted as an ID.",
	Serialize:  coerceString,
	ParseValue: coerceString,
	ParseLiteral: func(valueAST ast.Value) any {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			return valueAST.Value
		case *ast.StringValue:
			return valueAST.Value
		}
		return nil
	},
})
