package graphql

import (
	"fmt"
	"math"
	"strconv"

	"github.com/sprucehealth/graphql/language/ast"
)

func coerceInt(value any) (any, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case int:
		return value, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		if v < int64(math.MinInt32) || v > int64(math.MaxInt32) {
			return nil, nil
		}
		return int(v), nil
	case uint:
		//nolint:gosec
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		if v > uint32(math.MaxInt32) {
			return nil, nil
		}
		return int(v), nil
	case uint64:
		if v > uint64(math.MaxInt32) {
			return nil, nil
		}
		return int(v), nil
	case float32:
		if v < float32(math.MinInt32) || v > float32(math.MaxInt32) {
			return nil, nil
		}
		return int(v), nil
	case float64:
		if v < float64(math.MinInt64) || v > float64(math.MaxInt64) {
			return nil, nil
		}
		return int(v), nil
	case string:
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, nil
		}
		return int(val), nil
	}

	// If the value cannot be transformed into an int, return nil instead of '0'
	// to denote 'no integer found'
	return nil, nil
}

// Int is the GraphQL Integer type definition.
var Int = NewScalar(ScalarConfig{
	Name: "Int",
	Description: "The `Int` scalar type represents non-fractional signed whole numeric " +
		"values. Int can represent values between -(2^31) and 2^31 - 1. ",
	Serialize:  coerceInt,
	ParseValue: coerceInt,
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			if intValue, err := strconv.Atoi(valueAST.Value); err == nil {
				return intValue, nil
			}
		}
		return nil, nil
	},
})

func coerceFloat64(value any) (any, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return float64(1), nil
		}
		return float64(0), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return value, nil
	case string:
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, nil
		}
		return val, nil
	}
	return float64(0), nil
}

// Float is the GraphQL float type definition.
var Float = NewScalar(ScalarConfig{
	Name: "Float",
	Description: "The `Float` scalar type represents signed double-precision fractional " +
		"values as specified by " +
		"[IEEE 754](http://en.wikipedia.org/wiki/IEEE_floating_point). ",
	Serialize:  coerceFloat64,
	ParseValue: coerceFloat64,
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		switch valueAST := valueAST.(type) {
		case *ast.FloatValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue, nil
			}
		case *ast.IntValue:
			if floatValue, err := strconv.ParseFloat(valueAST.Value, 64); err == nil {
				return floatValue, nil
			}
		}
		return nil, nil
	},
})

func coerceString(value any) (any, error) {
	switch v := value.(type) {
	case string:
		return value, nil
	case int:
		return strconv.Itoa(v), nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	}
	return fmt.Sprintf("%v", value), nil
}

// String is the GraphQL string type definition
var String = NewScalar(ScalarConfig{
	Name: "String",
	Description: "The `String` scalar type represents textual data, represented as UTF-8 " +
		"character sequences. The String type is most often used by GraphQL to " +
		"represent free-form human-readable text.",
	Serialize:  coerceString,
	ParseValue: coerceString,
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			return valueAST.Value, nil
		}
		return nil, nil
	},
})

func coerceBool(value any) (any, error) {
	switch v := value.(type) {
	case bool:
		return value, nil
	case string:
		switch v {
		case "", "false":
			return false, nil
		}
		return true, nil
	case float64:
		return v != 0.0, nil
	case float32:
		return v != 0.0, nil
	case int:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	}
	return false, nil
}

// Boolean is the GraphQL boolean type definition
var Boolean = NewScalar(ScalarConfig{
	Name:        "Boolean",
	Description: "The `Boolean` scalar type represents `true` or `false`.",
	Serialize:   coerceBool,
	ParseValue:  coerceBool,
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		switch valueAST := valueAST.(type) {
		case *ast.BooleanValue:
			return valueAST.Value, nil
		}
		return nil, nil
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
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		switch valueAST := valueAST.(type) {
		case *ast.IntValue:
			return valueAST.Value, nil
		case *ast.StringValue:
			return valueAST.Value, nil
		}
		return nil, nil
	},
})
