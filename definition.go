package graphql

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
)

// Type interface for all of the possible kinds of GraphQL types
type Type interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Type = (*Scalar)(nil)
var _ Type = (*Object)(nil)
var _ Type = (*Interface)(nil)
var _ Type = (*Union)(nil)
var _ Type = (*Enum)(nil)
var _ Type = (*InputObject)(nil)
var _ Type = (*List)(nil)
var _ Type = (*NonNull)(nil)
var _ Type = (*Argument)(nil)

// Input interface for types that may be used as input types for arguments and directives.
type Input interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Input = (*Scalar)(nil)
var _ Input = (*Enum)(nil)
var _ Input = (*InputObject)(nil)
var _ Input = (*List)(nil)
var _ Input = (*NonNull)(nil)

// IsInputType determines if given type is a GraphQLInputType
func IsInputType(ttype Type) bool {
	named := GetNamed(ttype)
	if _, ok := named.(*Scalar); ok {
		return true
	}
	if _, ok := named.(*Enum); ok {
		return true
	}
	if _, ok := named.(*InputObject); ok {
		return true
	}
	return false
}

// IsOutputType determines if given type is a GraphQLOutputType
func IsOutputType(ttype Type) bool {
	name := GetNamed(ttype)
	if _, ok := name.(*Scalar); ok {
		return true
	}
	if _, ok := name.(*Object); ok {
		return true
	}
	if _, ok := name.(*Interface); ok {
		return true
	}
	if _, ok := name.(*Union); ok {
		return true
	}
	if _, ok := name.(*Enum); ok {
		return true
	}
	return false
}

// Leaf interface for types that may be leaf values
type Leaf interface {
	Name() string
	Description() string
	String() string
	Error() error
	Serialize(value any) any
}

var _ Leaf = (*Scalar)(nil)
var _ Leaf = (*Enum)(nil)

// IsLeafType determines if given type is a leaf value
func IsLeafType(ttype Type) bool {
	named := GetNamed(ttype)
	if _, ok := named.(*Scalar); ok {
		return true
	}
	if _, ok := named.(*Enum); ok {
		return true
	}
	return false
}

// Output interface for types that may be used as output types as the result of fields.
type Output interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Output = (*Scalar)(nil)
var _ Output = (*Object)(nil)
var _ Output = (*Interface)(nil)
var _ Output = (*Union)(nil)
var _ Output = (*Enum)(nil)
var _ Output = (*List)(nil)
var _ Output = (*NonNull)(nil)

// Composite interface for types that may describe the parent context of a selection set.
type Composite interface {
	Name() string
	Description() string
	String() string
	Error() error
}

var _ Composite = (*Object)(nil)
var _ Composite = (*Interface)(nil)
var _ Composite = (*Union)(nil)

// IsCompositeType determines if given type is a GraphQLComposite type
func IsCompositeType(ttype any) bool {
	if _, ok := ttype.(*Object); ok {
		return true
	}
	if _, ok := ttype.(*Interface); ok {
		return true
	}
	if _, ok := ttype.(*Union); ok {
		return true
	}
	return false
}

// Abstract interface for types that may describe the parent context of a selection set.
type Abstract interface {
	Name() string
}

var _ Abstract = (*Interface)(nil)
var _ Abstract = (*Union)(nil)

func IsAbstractType(ttype any) bool {
	if _, ok := ttype.(*Interface); ok {
		return true
	}
	if _, ok := ttype.(*Union); ok {
		return true
	}
	return false
}

// Nullable interface for types that can accept null as a value.
type Nullable interface {
}

var _ Nullable = (*Scalar)(nil)
var _ Nullable = (*Object)(nil)
var _ Nullable = (*Interface)(nil)
var _ Nullable = (*Union)(nil)
var _ Nullable = (*Enum)(nil)
var _ Nullable = (*InputObject)(nil)
var _ Nullable = (*List)(nil)

// GetNullable returns the Nullable type of the given GraphQL type
func GetNullable(ttype Type) Nullable {
	if ttype, ok := ttype.(*NonNull); ok {
		return ttype.OfType
	}
	return ttype
}

// Named interface for types that do not include modifiers like List or NonNull.
type Named interface {
	String() string
}

var _ Named = (*Scalar)(nil)
var _ Named = (*Object)(nil)
var _ Named = (*Interface)(nil)
var _ Named = (*Union)(nil)
var _ Named = (*Enum)(nil)
var _ Named = (*InputObject)(nil)

// GetNamed returns the Named type of the given GraphQL type
func GetNamed(ttype Type) Named {
	unmodifiedType := ttype
	for {
		if ttype, ok := unmodifiedType.(*List); ok {
			unmodifiedType = ttype.OfType
			continue
		}
		if ttype, ok := unmodifiedType.(*NonNull); ok {
			unmodifiedType = ttype.OfType
			continue
		}
		break
	}
	return unmodifiedType
}

// Scalar Type Definition
//
// The leaf values of any request and input values to arguments are
// Scalars (or Enums) and are defined with a name and a series of functions
// used to parse input from ast or variables and to ensure validity.
//
// Example:
//
//	var OddType = new Scalar({
//	  name: 'Odd',
//	  serialize(value) {
//	    return value % 2 === 1 ? value : null;
//	  }
//	});
type Scalar struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	scalarConfig ScalarConfig
	err          error
}

// SerializeFn is a function type for serializing a GraphQLScalar type value
type SerializeFn func(value any) any

// ParseValueFn is a function type for parsing the value of a GraphQLScalar type
type ParseValueFn func(value any) any

// ParseLiteralFn is a function type for parsing the literal value of a GraphQLScalar type
type ParseLiteralFn func(valueAST ast.Value) any

// ScalarConfig options for creating a new GraphQLScalar
type ScalarConfig struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Serialize    SerializeFn
	ParseValue   ParseValueFn
	ParseLiteral ParseLiteralFn
}

// NewScalar creates a new GraphQLScalar
func NewScalar(config ScalarConfig) *Scalar {
	st := &Scalar{}
	if config.Name == "" {
		st.err = gqlerrors.NewFormattedError("Type must be named.")
		return st
	}

	if err := assertValidName(config.Name); err != nil {
		st.err = err
		return st
	}

	st.PrivateName = config.Name
	st.PrivateDescription = config.Description

	if config.Serialize == nil {
		st.err = gqlerrors.NewFormattedError(fmt.Sprintf(`%v must provide "serialize" function. If this custom Scalar is `+
			`also used as an input type, ensure "parseValue" and "parseLiteral" `+
			`functions are also provided.`, st))
		return st
	}
	if config.ParseValue != nil || config.ParseLiteral != nil {
		if config.ParseValue == nil || config.ParseLiteral == nil {
			st.err = gqlerrors.NewFormattedError(fmt.Sprintf(`%v must provide both "parseValue" and "parseLiteral" functions.`, st))
			return st
		}
	}

	st.scalarConfig = config
	return st
}
func (st *Scalar) Serialize(value any) any {
	if st.scalarConfig.Serialize == nil {
		return value
	}
	return st.scalarConfig.Serialize(value)
}
func (st *Scalar) ParseValue(value any) any {
	if st.scalarConfig.ParseValue == nil {
		return value
	}
	return st.scalarConfig.ParseValue(value)
}
func (st *Scalar) ParseLiteral(valueAST ast.Value) any {
	if st.scalarConfig.ParseLiteral == nil {
		return nil
	}
	return st.scalarConfig.ParseLiteral(valueAST)
}
func (st *Scalar) Name() string {
	return st.PrivateName
}
func (st *Scalar) Description() string {
	return st.PrivateDescription
}
func (st *Scalar) String() string {
	return st.PrivateName
}
func (st *Scalar) Error() error {
	return st.err
}

// Object Type Definition
//
// Almost all of the GraphQL types you define will be object  Object types
// have a name, but most importantly describe their fields.
// Example:
//
//	var AddressType = new Object({
//	  name: 'Address',
//	  fields: {
//	    street: { type: String },
//	    number: { type: Int },
//	    formatted: {
//	      type: String,
//	      resolve(obj) {
//	        return obj.number + ' ' + obj.street
//	      }
//	    }
//	  }
//	});
//
// When two types need to refer to each other, or a type needs to refer to
// itself in a field, you can use a function expression (aka a closure or a
// thunk) to supply the fields lazily.
//
// Example:
//
//	var PersonType = new Object({
//	  name: 'Person',
//	  fields: () => ({
//	    name: { type: String },
//	    bestFriend: { type: PersonType },
//	  })
//	});
//
// /
type Object struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	IsTypeOf           IsTypeOfFn

	mu         sync.RWMutex
	typeConfig ObjectConfig
	fields     FieldDefinitionMap
	interfaces []*Interface
	// Interim alternative to throwing an error during schema definition at run-time
	err atomic.Value
}

// IsTypeOfParams Params for IsTypeOfFn()
type IsTypeOfParams struct {
	// Value that needs to be resolve.
	// Use this to decide which GraphQLObject this value maps to.
	Value any

	// Info is a collection of information about the current execution state.
	Info ResolveInfo
}

type IsTypeOfFn func(p IsTypeOfParams) bool

type InterfacesThunk func() []*Interface

type ObjectConfig struct {
	Name        string     `json:"name"`
	Interfaces  any        `json:"interfaces"`
	Fields      any        `json:"fields"`
	IsTypeOf    IsTypeOfFn `json:"isTypeOf"`
	Description string     `json:"description"`
}
type FieldsThunk func() Fields

type errWrapper struct{ err error }

func NewObject(config ObjectConfig) *Object {
	objectType := &Object{
		PrivateName:        config.Name,
		PrivateDescription: config.Description,
		IsTypeOf:           config.IsTypeOf,
		typeConfig:         config,
	}
	objectType.setErr(nil)

	if config.Name == "" {
		objectType.setErr(gqlerrors.NewFormattedError("Type must be named."))
		return objectType
	}
	if err := assertValidName(config.Name); err != nil {
		objectType.setErr(err)
		return objectType
	}

	return objectType
}
func (gt *Object) setErr(err error) {
	gt.err.Store(errWrapper{err: err})
}
func (gt *Object) AddFieldConfig(fieldName string, fieldConfig *Field) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if fields, ok := gt.typeConfig.Fields.(Fields); ok {
		fields[fieldName] = fieldConfig
		gt.fields = nil // invalidate the fields map cache
	}
}
func (gt *Object) Name() string {
	return gt.PrivateName
}
func (gt *Object) Description() string {
	return ""
}
func (gt *Object) String() string {
	return gt.PrivateName
}
func (gt *Object) Fields() FieldDefinitionMap {
	gt.mu.RLock()
	fields := gt.fields
	gt.mu.RUnlock()

	if fields != nil {
		return fields
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()
	fields = gt.fields
	if fields != nil {
		return fields
	}

	var configureFields Fields
	switch fields := gt.typeConfig.Fields.(type) {
	case Fields:
		configureFields = fields
	case FieldsThunk:
		configureFields = fields()
	}
	fields, err := defineFieldMap(gt, configureFields)
	gt.setErr(err)
	gt.fields = fields
	return gt.fields
}

func (gt *Object) Interfaces() []*Interface {
	gt.mu.RLock()
	interfaces := gt.interfaces
	gt.mu.RUnlock()

	if interfaces != nil {
		return interfaces
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()
	interfaces = gt.interfaces
	if interfaces != nil {
		return interfaces
	}

	var configInterfaces []*Interface
	switch ifs := gt.typeConfig.Interfaces.(type) {
	case InterfacesThunk:
		configInterfaces = ifs()
	case []*Interface:
		configInterfaces = ifs
	case nil:
	default:
		gt.setErr(fmt.Errorf("Unknown Object.Interfaces type: %v", reflect.TypeOf(gt.typeConfig.Interfaces)))
		return nil
	}
	interfaces, err := defineInterfaces(gt, configInterfaces)
	gt.setErr(err)
	gt.interfaces = interfaces
	return gt.interfaces
}
func (gt *Object) Error() error {
	return gt.err.Load().(errWrapper).err
}

func defineInterfaces(ttype *Object, interfaces []*Interface) ([]*Interface, error) {
	if len(interfaces) == 0 {
		return nil, nil
	}
	ifaces := make([]*Interface, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface == nil {
			return ifaces, gqlerrors.NewFormattedError(fmt.Sprintf(`%v may only implement Interface types, it cannot implement: %v.`, ttype, iface))
		}
		if iface.ResolveType != nil {
			if iface.ResolveType == nil {
				return ifaces, gqlerrors.NewFormattedError(fmt.Sprintf(`Interface Type %v does not provide a "resolveType" function `+
					`and implementing Type %v does not provide a "isTypeOf" `+
					`function. There is no way to resolve this implementing type `+
					`during execution.`, iface, ttype))
			}
		}
		ifaces = append(ifaces, iface)
	}
	return ifaces, nil
}

func defineFieldMap(ttype Named, fields Fields) (FieldDefinitionMap, error) {
	if len(fields) == 0 {
		return nil, gqlerrors.NewFormattedError(fmt.Sprintf(`%v fields must be an object with field names as keys or a function which return such an object.`, ttype))
	}

	resultFieldMap := make(FieldDefinitionMap, len(fields))
	for fieldName, field := range fields {
		if field == nil {
			continue
		}
		if field.Type == nil {
			return resultFieldMap, gqlerrors.NewFormattedError(fmt.Sprintf(`%v.%v field type must be Output Type but got: %v.`, ttype, fieldName, field.Type))
		}
		if field.Type.Error() != nil {
			return resultFieldMap, field.Type.Error()
		}
		if err := assertValidName(fieldName); err != nil {
			return resultFieldMap, err
		}
		fieldDef := &FieldDefinition{
			Name:              fieldName,
			Description:       field.Description,
			Type:              field.Type,
			Resolve:           field.Resolve,
			DeprecationReason: field.DeprecationReason,
			Directives:        field.Directives,
		}

		if len(field.Args) != 0 {
			fieldDef.Args = make([]*Argument, 0, len(field.Args))
			for argName, arg := range field.Args {
				err := assertValidName(argName)
				if err != nil {
					return resultFieldMap, err
				}
				if arg == nil {
					return resultFieldMap, gqlerrors.NewFormattedError(fmt.Sprintf(`%v.%v args must be an object with argument names as keys.`, ttype, fieldName))
				}
				if arg.Type == nil {
					return resultFieldMap, gqlerrors.NewFormattedError(fmt.Sprintf(`%v.%v(%v:) argument type must be Input Type but got: %v.`, ttype, fieldName, argName, arg.Type))
				}
				fieldArg := &Argument{
					PrivateName:        argName,
					PrivateDescription: arg.Description,
					Type:               arg.Type,
					DefaultValue:       arg.DefaultValue,
				}
				fieldDef.Args = append(fieldDef.Args, fieldArg)
			}
		}
		resultFieldMap[fieldName] = fieldDef
	}
	return resultFieldMap, nil
}

// ResolveParams Params for FieldResolveFn()
type ResolveParams struct {
	// Source is the source value
	Source any

	// Args is a map of arguments for current GraphQL request
	Args map[string]any

	// Info is a collection of information about the current execution state.
	Info ResolveInfo
}

type FieldResolveFn func(ctx context.Context, p ResolveParams) (any, error)

type ResolveInfo struct {
	FieldName      string
	FieldASTs      []*ast.Field
	ReturnType     Output
	ParentType     Composite
	Schema         Schema
	Fragments      map[string]*ast.FragmentDefinition
	RootValue      any
	Operation      ast.Definition
	VariableValues map[string]any
}

type Fields map[string]*Field

type Field struct {
	Name              string              `json:"name"` // used by graphlql-relay
	Type              Output              `json:"type"`
	Args              FieldConfigArgument `json:"args"`
	Resolve           FieldResolveFn
	DeprecationReason string           `json:"deprecationReason,omitempty"`
	Description       string           `json:"description"`
	Directives        []*ast.Directive `json:"directives,omitempty"`
}

type FieldConfigArgument map[string]*ArgumentConfig

type ArgumentConfig struct {
	Type         Input  `json:"type"`
	DefaultValue any    `json:"defaultValue"`
	Description  string `json:"description"`
}

type FieldDefinitionMap map[string]*FieldDefinition
type FieldDefinition struct {
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Type              Output           `json:"type"`
	Args              []*Argument      `json:"args"`
	Resolve           FieldResolveFn   `json:"-"`
	DeprecationReason string           `json:"deprecationReason,omitempty"`
	Directives        []*ast.Directive `json:"directives,omitempty"`
}

type FieldArgument struct {
	Name         string `json:"name"`
	Type         Type   `json:"type"`
	DefaultValue any    `json:"defaultValue"`
	Description  string `json:"description"`
}

type Argument struct {
	PrivateName        string `json:"name"`
	Type               Input  `json:"type"`
	DefaultValue       any    `json:"defaultValue"`
	PrivateDescription string `json:"description"`
}

func (st *Argument) Name() string {
	return st.PrivateName
}
func (st *Argument) Description() string {
	return st.PrivateDescription
}
func (st *Argument) String() string {
	return st.PrivateName
}
func (st *Argument) Error() error {
	return nil
}

// Interface Type Definition
//
// When a field can return one of a heterogeneous set of types, a Interface type
// is used to describe what types are possible, what fields are in common across
// all types, as well as a function to determine which type is actually used
// when the field is resolved.
//
// Example:
//
//	var EntityType = new Interface({
//	  name: 'Entity',
//	  fields: {
//	    name: { type: String }
//	  }
//	});
type Interface struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	ResolveType        ResolveTypeFn

	mu         sync.RWMutex
	typeConfig InterfaceConfig
	fields     FieldDefinitionMap

	muErr sync.RWMutex
	err   error
}

type InterfaceConfig struct {
	Name        string `json:"name"`
	Fields      any    `json:"fields"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}

// ResolveTypeParams Params for ResolveTypeFn()
type ResolveTypeParams struct {
	// Value that needs to be resolve.
	// Use this to decide which GraphQLObject this value maps to.
	Value any

	// Info is a collection of information about the current execution state.
	Info ResolveInfo
}

type ResolveTypeFn func(ctx context.Context, p ResolveTypeParams) *Object

func NewInterface(config InterfaceConfig) *Interface {
	it := &Interface{
		PrivateName:        config.Name,
		PrivateDescription: config.Description,
		ResolveType:        config.ResolveType,
		typeConfig:         config,
	}
	if config.Name == "" {
		it.err = gqlerrors.NewFormattedError("Type must be named.")
		return it
	}
	if err := assertValidName(config.Name); err != nil {
		it.err = err
		return it
	}
	return it
}

func (it *Interface) AddFieldConfig(fieldName string, fieldConfig *Field) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	it.mu.Lock()
	defer it.mu.Unlock()
	if fields, ok := it.typeConfig.Fields.(Fields); ok {
		fields[fieldName] = fieldConfig
		it.fields = nil
	}
}

func (it *Interface) Name() string {
	return it.PrivateName
}

func (it *Interface) Description() string {
	return it.PrivateDescription
}

func (it *Interface) Fields() FieldDefinitionMap {
	it.mu.RLock()
	fields := it.fields
	it.mu.RUnlock()
	if fields != nil {
		return fields
	}

	it.mu.Lock()
	defer it.mu.Unlock()

	var configureFields Fields
	switch fields := it.typeConfig.Fields.(type) {
	case Fields:
		configureFields = fields
	case FieldsThunk:
		configureFields = fields()
	}
	fields, err := defineFieldMap(it, configureFields)
	it.fields = fields
	it.muErr.Lock()
	it.err = err
	it.muErr.Unlock()
	return it.fields
}

func (it *Interface) String() string {
	return it.PrivateName
}
func (it *Interface) Error() error {
	it.muErr.RLock()
	defer it.muErr.RUnlock()
	return it.err
}

// Union Type Definition
//
// When a field can return one of a heterogeneous set of types, a Union type
// is used to describe what types are possible as well as providing a function
// to determine which type is actually used when the field is resolved.
//
// Example:
//
//	var PetType = new Union({
//	  name: 'Pet',
//	  types: [ DogType, CatType ],
//	  resolveType(value) {
//	    if (value instanceof Dog) {
//	      return DogType;
//	    }
//	    if (value instanceof Cat) {
//	      return CatType;
//	    }
//	  }
//	});
type Union struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`
	ResolveType        ResolveTypeFn

	typeConfig UnionConfig
	types      []*Object

	err error
}
type UnionConfig struct {
	Name        string    `json:"name"`
	Types       []*Object `json:"types"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}

func NewUnion(config UnionConfig) *Union {
	objectType := &Union{
		PrivateName:        config.Name,
		PrivateDescription: config.Description,
		ResolveType:        config.ResolveType,
	}
	if config.Name == "" {
		objectType.err = gqlerrors.NewFormattedError("Type must be named.")
		return objectType
	}
	if err := assertValidName(config.Name); err != nil {
		objectType.err = err
		return objectType
	}

	if len(config.Types) == 0 {
		objectType.err = gqlerrors.NewFormattedError(fmt.Sprintf(`Must provide Array of types for Union %v.`, config.Name))
		return objectType
	}
	for _, ttype := range config.Types {
		if ttype == nil {
			objectType.err = gqlerrors.NewFormattedError(fmt.Sprintf(`%v may only contain Object types, it cannot contain: %v.`, objectType, ttype))
			return objectType
		}
		if objectType.ResolveType == nil {
			if ttype.IsTypeOf == nil {
				objectType.err = gqlerrors.NewFormattedError(fmt.Sprintf(`Union Type %v does not provide a "resolveType" function `+
					`and possible Type %v does not provide a "isTypeOf" `+
					`function. There is no way to resolve this possible type `+
					`during execution.`, objectType, ttype))
				return objectType
			}
		}
	}
	objectType.types = config.Types
	objectType.typeConfig = config
	return objectType
}
func (ut *Union) Types() []*Object {
	return ut.types
}

func (ut *Union) String() string {
	return ut.PrivateName
}
func (ut *Union) Name() string {
	return ut.PrivateName
}
func (ut *Union) Description() string {
	return ut.PrivateDescription
}
func (ut *Union) Error() error {
	return ut.err
}

// Enum Type Definition
//
// Some leaf values of requests and input values are Enums. GraphQL serializes
// Enum values as strings, however internally Enums can be represented by any
// kind of type, often integers.
//
// Example:
//
//     var RGBType = new Enum({
//       name: 'RGB',
//       values: {
//         RED: { value: 0 },
//         GREEN: { value: 1 },
//         BLUE: { value: 2 }
//       }
//     });
//
// Note: If a value is not provided in a definition, the name of the enum value
// will be used as its internal value.

type Enum struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	enumConfig EnumConfig
	values     []*EnumValueDefinition

	mu           sync.RWMutex
	valuesLookup map[any]*EnumValueDefinition
	nameLookup   map[string]*EnumValueDefinition
	err          error
}
type EnumValueConfigMap map[string]*EnumValueConfig
type EnumValueConfig struct {
	Value             any    `json:"value"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
	Description       string `json:"description"`
}
type EnumConfig struct {
	Name        string             `json:"name"`
	Values      EnumValueConfigMap `json:"values"`
	Description string             `json:"description"`
}
type EnumValueDefinition struct {
	Name              string `json:"name"`
	Value             any    `json:"value"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
	Description       string `json:"description"`
}

func NewEnum(config EnumConfig) *Enum {
	gt := &Enum{}
	gt.enumConfig = config

	err := assertValidName(config.Name)
	if err != nil {
		gt.err = err
		return gt
	}

	gt.PrivateName = config.Name
	gt.PrivateDescription = config.Description
	gt.values, err = gt.defineEnumValues(config.Values)
	if err != nil {
		gt.err = err
		return gt
	}

	return gt
}
func (gt *Enum) defineEnumValues(valueMap EnumValueConfigMap) ([]*EnumValueDefinition, error) {
	if len(valueMap) == 0 {
		return nil, gqlerrors.NewFormattedError(fmt.Sprintf(`%v values must be an object with value names as keys.`, gt))
	}

	values := make([]*EnumValueDefinition, 0, len(valueMap))
	for valueName, valueConfig := range valueMap {
		if valueConfig == nil {
			return values, gqlerrors.NewFormattedError(fmt.Sprintf(`%v.%v must refer to an object with a "value" key `+
				`representing an internal value but got: %v.`, gt, valueName, valueConfig))
		}
		if err := assertValidName(valueName); err != nil {
			return values, err
		}
		value := &EnumValueDefinition{
			Name:              valueName,
			Value:             valueConfig.Value,
			DeprecationReason: valueConfig.DeprecationReason,
			Description:       valueConfig.Description,
		}
		if value.Value == nil {
			value.Value = valueName
		}
		values = append(values, value)
	}
	return values, nil
}
func (gt *Enum) Values() []*EnumValueDefinition {
	return gt.values
}
func (gt *Enum) Serialize(value any) any {
	if enumValue, ok := gt.getValueLookup()[value]; ok {
		return enumValue.Name
	}
	return nil
}
func (gt *Enum) ParseValue(value any) any {
	valueStr, ok := value.(string)
	if !ok {
		return nil
	}
	if enumValue, ok := gt.getNameLookup()[valueStr]; ok {
		return enumValue.Value
	}
	return nil
}
func (gt *Enum) ParseLiteral(valueAST ast.Value) any {
	if valueAST, ok := valueAST.(*ast.EnumValue); ok {
		if enumValue, ok := gt.getNameLookup()[valueAST.Value]; ok {
			return enumValue.Value
		}
	}
	return nil
}
func (gt *Enum) Name() string {
	return gt.PrivateName
}
func (gt *Enum) Description() string {
	return gt.PrivateDescription
}
func (gt *Enum) String() string {
	return gt.PrivateName
}
func (gt *Enum) Error() error {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	return gt.err
}
func (gt *Enum) getValueLookup() map[any]*EnumValueDefinition {
	gt.mu.RLock()
	valuesLookup := gt.valuesLookup
	gt.mu.RUnlock()
	if len(valuesLookup) != 0 {
		return valuesLookup
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()
	if len(gt.valuesLookup) != 0 {
		return gt.valuesLookup
	}
	valuesLookup = map[any]*EnumValueDefinition{}
	for _, value := range gt.Values() {
		valuesLookup[value.Value] = value
	}
	gt.valuesLookup = valuesLookup
	return valuesLookup
}

func (gt *Enum) getNameLookup() map[string]*EnumValueDefinition {
	gt.mu.RLock()
	nameLookup := gt.nameLookup
	gt.mu.RUnlock()
	if len(nameLookup) != 0 {
		return nameLookup
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()
	if len(gt.nameLookup) > 0 {
		return gt.nameLookup
	}
	nameLookup = map[string]*EnumValueDefinition{}
	for _, value := range gt.Values() {
		nameLookup[value.Name] = value
	}
	gt.nameLookup = nameLookup
	return nameLookup
}

// InputObject Type Definition
//
// An input object defines a structured collection of fields which may be
// supplied to a field argument.
//
// # Using `NonNull` will ensure that a value must be provided by the query
//
// Example:
//
//	var GeoPoint = new InputObject({
//	  name: 'GeoPoint',
//	  fields: {
//	    lat: { type: new NonNull(Float) },
//	    lon: { type: new NonNull(Float) },
//	    alt: { type: Float, defaultValue: 0 },
//	  }
//	});
type InputObject struct {
	PrivateName        string `json:"name"`
	PrivateDescription string `json:"description"`

	mu         sync.RWMutex
	typeConfig InputObjectConfig
	fields     InputObjectFieldMap

	err error
}
type InputObjectFieldConfig struct {
	Type              Input  `json:"type"`
	DefaultValue      any    `json:"defaultValue"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
	Description       string `json:"description"`
}

type InputObjectFields map[string]*InputObjectField

type InputObjectField struct {
	PrivateName        string `json:"name"`
	Type               Input  `json:"type"`
	DefaultValue       any    `json:"defaultValue"`
	PrivateDescription string `json:"description"`
	DeprecationReason  string `json:"deprecationReason,omitempty"`
}

func (st *InputObjectField) Name() string {
	return st.PrivateName
}
func (st *InputObjectField) Description() string {
	return st.PrivateDescription
}
func (st *InputObjectField) String() string {
	return st.PrivateName
}
func (st *InputObjectField) Error() error {
	return nil
}

type InputObjectConfigFieldMap map[string]*InputObjectFieldConfig
type InputObjectFieldMap map[string]*InputObjectField
type InputObjectConfigFieldMapThunk func() InputObjectConfigFieldMap
type InputObjectConfig struct {
	Name        string `json:"name"`
	Fields      any    `json:"fields"`
	Description string `json:"description"`
}

func NewInputObject(config InputObjectConfig) *InputObject {
	gt := &InputObject{}
	if config.Name == "" {
		gt.err = gqlerrors.NewFormattedError("Type must be named.")
		return gt
	}
	gt.PrivateName = config.Name
	gt.PrivateDescription = config.Description
	gt.typeConfig = config
	gt.mu.Lock()
	defer gt.mu.Unlock()
	gt.fields = gt.defineFieldMap()
	return gt
}
func (gt *InputObject) AddInputField(fieldName string, fieldConfig *InputObjectFieldConfig) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if fields, ok := gt.typeConfig.Fields.(InputObjectConfigFieldMap); ok {
		fields[fieldName] = fieldConfig
		gt.fields = nil // invalidate the fields map cache
	}
}

func (gt *InputObject) defineFieldMap() InputObjectFieldMap {
	var fieldMap InputObjectConfigFieldMap
	switch fields := gt.typeConfig.Fields.(type) {
	case InputObjectConfigFieldMap:
		fieldMap = fields
	case InputObjectConfigFieldMapThunk:
		fieldMap = fields()
	}
	resultFieldMap := InputObjectFieldMap{}

	if len(fieldMap) == 0 {
		gt.err = gqlerrors.NewFormattedError(fmt.Sprintf(`%v fields must be an object with field names as keys or a function which return such an object.`, gt))
		return resultFieldMap
	}

	for fieldName, fieldConfig := range fieldMap {
		if fieldConfig == nil {
			continue
		}
		if err := assertValidName(fieldName); err != nil {
			continue
		}
		if fieldConfig.Type == nil {
			gt.err = gqlerrors.NewFormattedError(fmt.Sprintf(`%v.%v field type must be Input Type but got: %v.`, gt, fieldName, fieldConfig.Type))
			return resultFieldMap
		}
		resultFieldMap[fieldName] = &InputObjectField{
			PrivateName:        fieldName,
			Type:               fieldConfig.Type,
			PrivateDescription: fieldConfig.Description,
			DefaultValue:       fieldConfig.DefaultValue,
			DeprecationReason:  fieldConfig.DeprecationReason,
		}
	}
	return resultFieldMap
}
func (gt *InputObject) Fields() InputObjectFieldMap {
	gt.mu.RLock()
	fields := gt.fields
	gt.mu.RUnlock()

	if fields != nil {
		return fields
	}

	gt.mu.Lock()
	defer gt.mu.Unlock()
	gt.fields = gt.defineFieldMap()
	return gt.fields
}
func (gt *InputObject) Name() string {
	return gt.PrivateName
}
func (gt *InputObject) Description() string {
	return gt.PrivateDescription
}
func (gt *InputObject) String() string {
	return gt.PrivateName
}
func (gt *InputObject) Error() error {
	return gt.err
}

// List Modifier
//
// A list is a kind of type marker, a wrapping type which points to another
// type. Lists are often created within the context of defining the fields of
// an object type.
//
// Example:
//
//	var PersonType = new Object({
//	  name: 'Person',
//	  fields: () => ({
//	    parents: { type: new List(Person) },
//	    children: { type: new List(Person) },
//	  })
//	})
type List struct {
	OfType Type `json:"ofType"`

	err error
}

func NewList(ofType Type) *List {
	gl := &List{}
	if ofType == nil {
		gl.err = gqlerrors.NewFormattedError(fmt.Sprintf(`Can only create List of a Type but got: %v.`, ofType))
		return gl
	}
	gl.OfType = ofType
	return gl
}
func (gl *List) Name() string {
	return fmt.Sprintf("%v", gl.OfType)
}
func (gl *List) Description() string {
	return ""
}
func (gl *List) String() string {
	if gl.OfType != nil {
		return fmt.Sprintf("[%v]", gl.OfType)
	}
	return ""
}
func (gl *List) Error() error {
	return gl.err
}

// NonNull Modifier
//
// A non-null is a kind of type marker, a wrapping type which points to another
// type. Non-null types enforce that their values are never null and can ensure
// an error is raised if this ever occurs during a request. It is useful for
// fields which you can make a strong guarantee on non-nullability, for example
// usually the id field of a database row will never be null.
//
// Example:
//
//	var RowType = new Object({
//	  name: 'Row',
//	  fields: () => ({
//	    id: { type: new NonNull(String) },
//	  })
//	})
//
// Note: the enforcement of non-nullability occurs within the executor.
type NonNull struct {
	OfType Type `json:"ofType"`

	err error
}

func NewNonNull(ofType Type) *NonNull {
	gl := &NonNull{}
	_, isOfTypeNonNull := ofType.(*NonNull)
	if ofType == nil || isOfTypeNonNull {
		gl.err = gqlerrors.NewFormattedError(fmt.Sprintf(`Can only create NonNull of a Nullable Type but got: %v.`, ofType))
		return gl
	}
	gl.OfType = ofType
	return gl
}
func (gl *NonNull) Name() string {
	return fmt.Sprintf("%v!", gl.OfType)
}
func (gl *NonNull) Description() string {
	return ""
}
func (gl *NonNull) String() string {
	if gl.OfType != nil {
		return gl.Name()
	}
	return ""
}
func (gl *NonNull) Error() error {
	return gl.err
}

var NameRegExp = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")

func assertValidName(name string) error {
	if !NameRegExp.MatchString(name) {
		return gqlerrors.NewFormattedError(fmt.Sprintf(`Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but "%v" does not.`, name))
	}
	return nil
}
