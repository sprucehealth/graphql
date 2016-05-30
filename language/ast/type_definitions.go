package ast

type TypeDefinition interface {
	GetOperation() string
	GetVariableDefinitions() []*VariableDefinition
	GetSelectionSet() *SelectionSet
	GetLoc() Location
}

var _ TypeDefinition = (*ScalarDefinition)(nil)
var _ TypeDefinition = (*ObjectDefinition)(nil)
var _ TypeDefinition = (*InterfaceDefinition)(nil)
var _ TypeDefinition = (*UnionDefinition)(nil)
var _ TypeDefinition = (*EnumDefinition)(nil)
var _ TypeDefinition = (*InputObjectDefinition)(nil)

type TypeSystemDefinition interface {
	GetOperation() string
	GetVariableDefinitions() []*VariableDefinition
	GetSelectionSet() *SelectionSet
	GetLoc() Location
}

var _ TypeSystemDefinition = (*SchemaDefinition)(nil)
var _ TypeSystemDefinition = (TypeDefinition)(nil)
var _ TypeSystemDefinition = (*TypeExtensionDefinition)(nil)
var _ TypeSystemDefinition = (*DirectiveDefinition)(nil)

// SchemaDefinition implements Node, Definition
type SchemaDefinition struct {
	Loc            Location
	OperationTypes []*OperationTypeDefinition
}

func (def *SchemaDefinition) GetLoc() Location {
	return def.Loc
}

func (def *SchemaDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *SchemaDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *SchemaDefinition) GetOperation() string {
	return ""
}

// ScalarDefinition implements Node, Definition
type OperationTypeDefinition struct {
	Loc       Location
	Operation string
	Type      *Named
}

func (def *OperationTypeDefinition) GetLoc() Location {
	return def.Loc
}

// ScalarDefinition implements Node, Definition
type ScalarDefinition struct {
	Loc  Location
	Name *Name
}

func (def *ScalarDefinition) GetLoc() Location {
	return def.Loc
}

func (def *ScalarDefinition) GetName() *Name {
	return def.Name
}

func (def *ScalarDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *ScalarDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *ScalarDefinition) GetOperation() string {
	return ""
}

// ObjectDefinition implements Node, Definition
type ObjectDefinition struct {
	Loc        Location
	Name       *Name
	Interfaces []*Named
	Fields     []*FieldDefinition
	Doc        *CommentGroup
}

func (def *ObjectDefinition) GetLoc() Location {
	return def.Loc
}

func (def *ObjectDefinition) GetName() *Name {
	return def.Name
}

func (def *ObjectDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *ObjectDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *ObjectDefinition) GetOperation() string {
	return ""
}

// FieldDefinition implements Node
type FieldDefinition struct {
	Loc       Location
	Name      *Name
	Arguments []*InputValueDefinition
	Type      Type
	Doc       *CommentGroup
	Comment   *CommentGroup
}

func (def *FieldDefinition) GetLoc() Location {
	return def.Loc
}

// InputValueDefinition implements Node
type InputValueDefinition struct {
	Loc          Location
	Name         *Name
	Type         Type
	DefaultValue Value
	Doc          *CommentGroup
	Comment      *CommentGroup
}

func (def *InputValueDefinition) GetLoc() Location {
	return def.Loc
}

// InterfaceDefinition implements Node, Definition
type InterfaceDefinition struct {
	Loc    Location
	Name   *Name
	Fields []*FieldDefinition
	Doc    *CommentGroup
}

func (def *InterfaceDefinition) GetLoc() Location {
	return def.Loc
}

func (def *InterfaceDefinition) GetName() *Name {
	return def.Name
}

func (def *InterfaceDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *InterfaceDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *InterfaceDefinition) GetOperation() string {
	return ""
}

// UnionDefinition implements Node, Definition
type UnionDefinition struct {
	Loc     Location
	Name    *Name
	Types   []*Named
	Doc     *CommentGroup
	Comment *CommentGroup
}

func (def *UnionDefinition) GetLoc() Location {
	return def.Loc
}

func (def *UnionDefinition) GetName() *Name {
	return def.Name
}

func (def *UnionDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *UnionDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *UnionDefinition) GetOperation() string {
	return ""
}

// EnumDefinition implements Node, Definition
type EnumDefinition struct {
	Loc    Location
	Name   *Name
	Values []*EnumValueDefinition
	Doc    *CommentGroup
}

func (def *EnumDefinition) GetLoc() Location {
	return def.Loc
}

func (def *EnumDefinition) GetName() *Name {
	return def.Name
}

func (def *EnumDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *EnumDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *EnumDefinition) GetOperation() string {
	return ""
}

// EnumValueDefinition implements Node, Definition
type EnumValueDefinition struct {
	Loc     Location
	Name    *Name
	Doc     *CommentGroup
	Comment *CommentGroup
}

func (def *EnumValueDefinition) GetLoc() Location {
	return def.Loc
}

// InputObjectDefinition implements Node, Definition
type InputObjectDefinition struct {
	Loc    Location
	Name   *Name
	Fields []*InputValueDefinition
	Doc    *CommentGroup
}

func (def *InputObjectDefinition) GetLoc() Location {
	return def.Loc
}

func (def *InputObjectDefinition) GetName() *Name {
	return def.Name
}

func (def *InputObjectDefinition) GetVariableDefinitions() []*VariableDefinition {
	return []*VariableDefinition{}
}

func (def *InputObjectDefinition) GetSelectionSet() *SelectionSet {
	return &SelectionSet{}
}

func (def *InputObjectDefinition) GetOperation() string {
	return ""
}
