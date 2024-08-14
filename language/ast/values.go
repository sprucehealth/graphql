package ast

type Value interface {
	GetValue() any
	GetLoc() Location
}

// Ensure that all value types implements Value interface
var _ Value = (*Variable)(nil)
var _ Value = (*IntValue)(nil)
var _ Value = (*FloatValue)(nil)
var _ Value = (*StringValue)(nil)
var _ Value = (*BooleanValue)(nil)
var _ Value = (*EnumValue)(nil)
var _ Value = (*ListValue)(nil)
var _ Value = (*ObjectValue)(nil)

// Variable implements Node, Value
type Variable struct {
	Loc  Location
	Name *Name
}

func (v *Variable) GetLoc() Location {
	return v.Loc
}

// GetValue alias to Variable.GetName()
func (v *Variable) GetValue() any {
	return v.GetName()
}

func (v *Variable) GetName() any {
	return v.Name
}

// IntValue implements Node, Value
type IntValue struct {
	Loc   Location
	Value string
}

func (v *IntValue) GetLoc() Location {
	return v.Loc
}

func (v *IntValue) GetValue() any {
	return v.Value
}

// FloatValue implements Node, Value
type FloatValue struct {
	Loc   Location
	Value string
}

func (v *FloatValue) GetLoc() Location {
	return v.Loc
}

func (v *FloatValue) GetValue() any {
	return v.Value
}

// StringValue implements Node, Value
type StringValue struct {
	Loc   Location
	Value string
}

func (v *StringValue) GetLoc() Location {
	return v.Loc
}

func (v *StringValue) GetValue() any {
	return v.Value
}

// BooleanValue implements Node, Value
type BooleanValue struct {
	Loc   Location
	Value bool
}

func (v *BooleanValue) GetLoc() Location {
	return v.Loc
}

func (v *BooleanValue) GetValue() any {
	return v.Value
}

// EnumValue implements Node, Value
type EnumValue struct {
	Loc   Location
	Value string
}

func (v *EnumValue) GetLoc() Location {
	return v.Loc
}

func (v *EnumValue) GetValue() any {
	return v.Value
}

// ListValue implements Node, Value
type ListValue struct {
	Loc    Location
	Values []Value
}

func (v *ListValue) GetLoc() Location {
	return v.Loc
}

// GetValue alias to ListValue.GetValues()
func (v *ListValue) GetValue() any {
	return v.GetValues()
}

func (v *ListValue) GetValues() any {
	// TODO: verify ObjectValue.GetValue()
	return v.Values
}

// ObjectValue implements Node, Value
type ObjectValue struct {
	Loc    Location
	Fields []*ObjectField
}

func (v *ObjectValue) GetLoc() Location {
	return v.Loc
}

func (v *ObjectValue) GetValue() any {
	// TODO: verify ObjectValue.GetValue()
	return v.Fields
}

// ObjectField implements Node, Value
type ObjectField struct {
	Name  *Name
	Loc   Location
	Value Value
}

func (f *ObjectField) GetLoc() Location {
	return f.Loc
}

func (f *ObjectField) GetValue() any {
	return f.Value
}
