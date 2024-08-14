package graphql

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/printer"
)

// Prepares an object map of variableValues of the correct type based on the
// provided variable definitions and arbitrary input. If the input cannot be
// parsed to match the variable definitions, a GraphQLError will be returned.
func getVariableValues(schema Schema, definitionASTs []*ast.VariableDefinition, inputs map[string]any) (map[string]any, error) {
	values := make(map[string]any, len(definitionASTs))
	for _, defAST := range definitionASTs {
		if defAST == nil || defAST.Variable == nil || defAST.Variable.Name == nil {
			continue
		}
		varName := defAST.Variable.Name.Value
		varValue, err := getVariableValue(schema, defAST, inputs[varName])
		if err != nil {
			return values, err
		}
		values[varName] = varValue
	}
	return values, nil
}

// Prepares an object map of argument values given a list of argument
// definitions and list of argument AST nodes.
func getArgumentValues(argDefs []*Argument, argASTs []*ast.Argument, variableVariables map[string]any) map[string]any {
	argASTMap := make(map[string]*ast.Argument, len(argASTs))
	for _, argAST := range argASTs {
		if argAST.Name != nil {
			argASTMap[argAST.Name.Value] = argAST
		}
	}
	results := make(map[string]any, len(argDefs))
	for _, argDef := range argDefs {
		name := argDef.PrivateName
		var valueAST ast.Value
		if argAST, ok := argASTMap[name]; ok {
			valueAST = argAST.Value
		}
		value := valueFromAST(valueAST, argDef.Type, variableVariables)
		if isNullish(value) {
			value = argDef.DefaultValue
		}
		if !isNullish(value) {
			results[name] = value
		}
	}
	return results
}

// Given a variable definition, and any value of input, return a value which
// adheres to the variable definition, or throw an error.
func getVariableValue(schema Schema, definitionAST *ast.VariableDefinition, input any) (any, error) {
	ttype, err := typeFromAST(schema, definitionAST.Type)
	if err != nil {
		return nil, err
	}
	variable := definitionAST.Variable

	if ttype == nil || !IsInputType(ttype) {
		return "", gqlerrors.NewError(
			gqlerrors.ErrorTypeInvalidInput,
			fmt.Sprintf(`Variable "$%v" expected value of type `+
				`"%v" which cannot be used as an input type.`, variable.Name.Value, printer.Print(definitionAST.Type)),
			[]ast.Node{definitionAST},
			"",
			nil,
			[]int{},
			nil,
		)
	}

	isValid, messages := isValidInputValue(input, ttype)
	if isValid {
		if isNullish(input) {
			defaultValue := definitionAST.DefaultValue
			if defaultValue != nil {
				variables := map[string]any{}
				val := valueFromAST(defaultValue, ttype, variables)
				return val, nil
			}
		}
		return coerceValue(ttype, input), nil
	}
	if isNullish(input) {
		return "", gqlerrors.NewError(
			gqlerrors.ErrorTypeInvalidInput,
			fmt.Sprintf(`Variable "$%v" of required type `+
				`"%v" was not provided.`, variable.Name.Value, printer.Print(definitionAST.Type)),
			[]ast.Node{definitionAST},
			"",
			nil,
			[]int{},
			nil,
		)
	}
	// convert input interface into string for error message
	var inputStr string
	b, err := json.Marshal(input)
	if err == nil {
		inputStr = string(b)
	}
	messagesStr := ""
	if len(messages) > 0 {
		messagesStr = "\n" + strings.Join(messages, "\n")
	}
	return "", gqlerrors.NewError(
		gqlerrors.ErrorTypeInvalidInput,
		fmt.Sprintf(`Variable "$%v" got invalid value `+
			`%v.%v`, variable.Name.Value, inputStr, messagesStr),
		[]ast.Node{definitionAST},
		"",
		nil,
		[]int{},
		nil,
	)
}

// Given a type and any value, return a runtime value coerced to match the type.
func coerceValue(ttype Input, value any) any {
	if ttype, ok := ttype.(*NonNull); ok {
		return coerceValue(ttype.OfType, value)
	}
	if isNullish(value) {
		return nil
	}
	if ttype, ok := ttype.(*List); ok {
		itemType := ttype.OfType
		valType := reflect.ValueOf(value)
		if valType.Kind() == reflect.Slice {
			values := []any{}
			for i := 0; i < valType.Len(); i++ {
				val := valType.Index(i).Interface()
				v := coerceValue(itemType, val)
				values = append(values, v)
			}
			return values
		}
		val := coerceValue(itemType, value)
		return []any{val}
	}
	if ttype, ok := ttype.(*InputObject); ok {
		valueMap, ok := value.(map[string]any)
		if !ok {
			valueMap = map[string]any{}
		}

		obj := map[string]any{}
		for fieldName, field := range ttype.Fields() {
			value := valueMap[fieldName]
			fieldValue := coerceValue(field.Type, value)
			if isNullish(fieldValue) {
				fieldValue = field.DefaultValue
			}
			if !isNullish(fieldValue) {
				obj[fieldName] = fieldValue
			}
		}
		return obj
	}

	switch ttype := ttype.(type) {
	case *Scalar:
		parsed := ttype.ParseValue(value)
		if !isNullish(parsed) {
			return parsed
		}
	case *Enum:
		parsed := ttype.ParseValue(value)
		if !isNullish(parsed) {
			return parsed
		}
	}
	return nil
}

// graphql-js/src/utilities.js`
// TODO: figure out where to organize utils
// TODO: change to *Schema
func typeFromAST(schema Schema, inputTypeAST ast.Type) (Type, error) {
	switch inputTypeAST := inputTypeAST.(type) {
	case *ast.List:
		innerType, err := typeFromAST(schema, inputTypeAST.Type)
		if err != nil {
			return nil, err
		}
		return NewList(innerType), nil
	case *ast.NonNull:
		innerType, err := typeFromAST(schema, inputTypeAST.Type)
		if err != nil {
			return nil, err
		}
		return NewNonNull(innerType), nil
	case *ast.Named:
		nameValue := ""
		if inputTypeAST.Name != nil {
			nameValue = inputTypeAST.Name.Value
		}
		ttype := schema.Type(nameValue)
		return ttype, nil
	default:
		if _, ok := inputTypeAST.(*ast.Named); !ok {
			return nil, gqlerrors.NewFormattedError("Must be a named type.")
		}
		return nil, nil
	}
}

// isValidInputValue alias isValidJSValue
// Given a value and a GraphQL type, determine if the value will be
// accepted for that type. This is primarily useful for validating the
// runtime values of query variables.
func isValidInputValue(value any, ttype Input) (bool, []string) {
	if ttype, ok := ttype.(*NonNull); ok {
		if isNullish(value) {
			if ttype.OfType.Name() != "" {
				return false, []string{fmt.Sprintf(`Expected "%v!", found null.`, ttype.OfType.Name())}
			}
			return false, []string{"Expected non-null value, found null."}
		}
		return isValidInputValue(value, ttype.OfType)
	}

	if isNullish(value) {
		return true, nil
	}

	switch ttype := ttype.(type) {
	case *List:
		itemType := ttype.OfType
		valType := reflect.ValueOf(value)
		if valType.Kind() == reflect.Ptr {
			valType = valType.Elem()
		}
		if valType.Kind() == reflect.Slice {
			var messagesReduce []string
			for i := 0; i < valType.Len(); i++ {
				val := valType.Index(i).Interface()
				_, messages := isValidInputValue(val, itemType)
				for idx, message := range messages {
					messagesReduce = append(messagesReduce, fmt.Sprintf(`In element #%v: %v`, idx+1, message))
				}
			}
			return len(messagesReduce) == 0, messagesReduce
		}
		return isValidInputValue(value, itemType)

	case *InputObject:
		valueMap, ok := value.(map[string]any)
		if !ok {
			return false, []string{fmt.Sprintf(`Expected "%v", found not an object.`, ttype.Name())}
		}
		fields := ttype.Fields()

		// to ensure stable order of field evaluation

		fieldNames := make([]string, 0, len(fields))
		for fieldName := range fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)

		valueMapFieldNames := make([]string, 0, len(valueMap))
		for fieldName := range valueMap {
			valueMapFieldNames = append(valueMapFieldNames, fieldName)
		}
		sort.Strings(valueMapFieldNames)

		var messagesReduce []string

		// Ensure every provided field is defined.
		for _, fieldName := range valueMapFieldNames {
			if _, ok := fields[fieldName]; !ok {
				messagesReduce = append(messagesReduce, fmt.Sprintf(`In field "%v": Unknown field.`, fieldName))
			}
		}
		// Ensure every defined field is valid.
		for _, fieldName := range fieldNames {
			_, messages := isValidInputValue(valueMap[fieldName], fields[fieldName].Type)
			for _, message := range messages {
				messagesReduce = append(messagesReduce, fmt.Sprintf(`In field "%v": %v`, fieldName, message))
			}
		}

		return len(messagesReduce) == 0, messagesReduce
	}

	switch ttype := ttype.(type) {
	case *Scalar:
		parsedVal := ttype.ParseValue(value)
		if isNullish(parsedVal) {
			return false, []string{fmt.Sprintf(`Expected type "%v", found "%v".`, ttype.Name(), value)}
		}
		return true, nil

	case *Enum:
		parsedVal := ttype.ParseValue(value)
		if isNullish(parsedVal) {
			return false, []string{fmt.Sprintf(`Expected type "%v", found "%v".`, ttype.Name(), value)}
		}
		return true, nil
	}
	return true, nil
}

// Returns true if a value is null, undefined, or NaN.
func isNullish(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case float32:
		return math.IsNaN(float64(v))
	case float64:
		return math.IsNaN(v)
	}
	// The any can hide an underlying nil ptr
	if v := reflect.ValueOf(value); v.Kind() == reflect.Ptr {
		return v.IsNil()
	}
	return false
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

/**
 * Produces a value given a GraphQL Value AST.
 *
 * A GraphQL type must be provided, which will be used to interpret different
 * GraphQL Value literals.
 *
 * | GraphQL Value        | JSON Value    |
 * | -------------------- | ------------- |
 * | Input Object         | Object        |
 * | List                 | Array         |
 * | Boolean              | Boolean       |
 * | String / Enum Value  | String        |
 * | Int / Float          | Number        |
 *
 */
func valueFromAST(valueAST ast.Value, ttype Input, variables map[string]any) any {
	if ttype, ok := ttype.(*NonNull); ok {
		val := valueFromAST(valueAST, ttype.OfType, variables)
		return val
	}

	if valueAST == nil {
		return nil
	}

	if valueAST, ok := valueAST.(*ast.Variable); ok {
		if valueAST.Name == nil {
			return nil
		}
		if variables == nil {
			return nil
		}
		variableName := valueAST.Name.Value
		variableVal, ok := variables[variableName]
		if !ok {
			return nil
		}
		// Note: we're not doing any checking that this variable is correct. We're
		// assuming that this query has been validated and the variable usage here
		// is of the correct type.
		return variableVal
	}

	if ttype, ok := ttype.(*List); ok {
		itemType := ttype.OfType
		if valueAST, ok := valueAST.(*ast.ListValue); ok {
			values := []any{}
			for _, itemAST := range valueAST.Values {
				v := valueFromAST(itemAST, itemType, variables)
				values = append(values, v)
			}
			return values
		}
		v := valueFromAST(valueAST, itemType, variables)
		return []any{v}
	}

	if ttype, ok := ttype.(*InputObject); ok {
		valueAST, ok := valueAST.(*ast.ObjectValue)
		if !ok {
			return nil
		}
		fieldASTs := map[string]*ast.ObjectField{}
		for _, fieldAST := range valueAST.Fields {
			if fieldAST.Name == nil {
				continue
			}
			fieldName := fieldAST.Name.Value
			fieldASTs[fieldName] = fieldAST

		}
		obj := make(map[string]any)
		for fieldName, field := range ttype.Fields() {
			fieldAST, ok := fieldASTs[fieldName]
			if !ok || fieldAST == nil {
				continue
			}
			fieldValue := valueFromAST(fieldAST.Value, field.Type, variables)
			if isNullish(fieldValue) {
				fieldValue = field.DefaultValue
			}
			if !isNullish(fieldValue) {
				obj[fieldName] = fieldValue
			}
		}
		return obj
	}

	switch ttype := ttype.(type) {
	case *Scalar:
		parsed := ttype.ParseLiteral(valueAST)
		if !isNullish(parsed) {
			return parsed
		}
	case *Enum:
		parsed := ttype.ParseLiteral(valueAST)
		if !isNullish(parsed) {
			return parsed
		}
	}
	return nil
}
