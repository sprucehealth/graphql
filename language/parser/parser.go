package parser

import (
	"fmt"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/lexer"
	"github.com/sprucehealth/graphql/language/source"
)

type parseFn func() (interface{}, error)

type ParseOptions struct {
	NoSource     bool
	KeepComments bool
}

type ParseParams struct {
	Source  interface{}
	Options ParseOptions
}

type Parser struct {
	Lexer   *lexer.Lexer
	Source  *source.Source
	Options ParseOptions

	prevEnd     int
	tok         lexer.Token
	comments    []*ast.CommentGroup
	leadComment *ast.CommentGroup
	lineComment *ast.CommentGroup
}

func Parse(p ParseParams) (*ast.Document, error) {
	var sourceObj *source.Source
	switch p.Source.(type) {
	case *source.Source:
		sourceObj = p.Source.(*source.Source)
	default:
		body, _ := p.Source.(string)
		sourceObj = source.New("GraphQL", body)
	}
	parser, err := makeParser(sourceObj, p.Options)
	if err != nil {
		return nil, err
	}
	doc, err := parser.parseDocument()
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Converts a name lex token into a name parse node.
func (p *Parser) parseName() (*ast.Name, error) {
	token, err := p.expect(lexer.NAME)
	if err != nil {
		return nil, err
	}
	return &ast.Name{
		Value: token.Value,
		Loc:   p.loc(token.Start),
	}, nil
}

func makeParser(s *source.Source, opts ParseOptions) (*Parser, error) {
	p := &Parser{
		Lexer:   lexer.New(s),
		Source:  s,
		Options: opts,
	}
	return p, p.next()
}

/* Implements the parsing rules in the Document section. */

func (p *Parser) parseDocument() (*ast.Document, error) {
	start := p.tok.Start
	var nodes []ast.Node
	for {
		if skp, err := p.skip(lexer.EOF); err != nil {
			return nil, err
		} else if skp {
			break
		}
		switch {
		case p.peek(lexer.BRACE_L):
			node, err := p.parseOperationDefinition()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		case p.peek(lexer.NAME):
			switch p.tok.Value {
			case "query", "mutation", "subscription": // Note: subscription is an experimental non-spec addition.
				node, err := p.parseOperationDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "fragment":
				node, err := p.parseFragmentDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "type":
				node, err := p.parseObjectTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "interface":
				node, err := p.parseInterfaceTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "union":
				node, err := p.parseUnionTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "scalar":
				node, err := p.parseScalarTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "enum":
				node, err := p.parseEnumTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "input":
				node, err := p.parseInputObjectTypeDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			case "extend":
				node, err := p.parseTypeExtensionDefinition()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
			default:
				if err := p.unexpected(lexer.Token{}); err != nil {
					return nil, err
				}
			}
		default:
			if err := p.unexpected(lexer.Token{}); err != nil {
				return nil, err
			}
		}
	}
	return &ast.Document{
		Loc:         p.loc(start),
		Definitions: nodes,
		Comments:    p.comments,
	}, nil
}

/* Implements the parsing rules in the Operations section. */

func (p *Parser) parseOperationDefinition() (*ast.OperationDefinition, error) {
	start := p.tok.Start
	if p.peek(lexer.BRACE_L) {
		selectionSet, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		return &ast.OperationDefinition{
			Operation:    "query",
			SelectionSet: selectionSet,
			Loc:          p.loc(start),
		}, nil
	}
	operationToken, err := p.expect(lexer.NAME)
	if err != nil {
		return nil, err
	}
	operation := ""
	switch operationToken.Value {
	case "mutation":
		fallthrough
	case "subscription":
		fallthrough
	case "query":
		operation = operationToken.Value
	default:
		return nil, p.unexpected(operationToken)
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	variableDefinitions, err := p.parseVariableDefinitions()
	if err != nil {
		return nil, err
	}
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	selectionSet, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	return &ast.OperationDefinition{
		Operation:           operation,
		Name:                name,
		VariableDefinitions: variableDefinitions,
		Directives:          directives,
		SelectionSet:        selectionSet,
		Loc:                 p.loc(start),
	}, nil
}

func (p *Parser) parseVariableDefinitions() ([]*ast.VariableDefinition, error) {
	if !p.peek(lexer.PAREN_L) {
		return nil, nil
	}
	vdefs, err := p.many(lexer.PAREN_L, p.parseVariableDefinition, lexer.PAREN_R)
	if err != nil {
		return nil, err
	}
	variableDefinitions := make([]*ast.VariableDefinition, 0, len(vdefs))
	for _, vdef := range vdefs {
		if vdef != nil {
			variableDefinitions = append(variableDefinitions, vdef.(*ast.VariableDefinition))
		}
	}
	return variableDefinitions, nil
}

func (p *Parser) parseVariableDefinition() (interface{}, error) {
	start := p.tok.Start
	variable, err := p.parseVariable()
	if err != nil {
		return nil, err
	}
	_, err = p.expect(lexer.COLON)
	if err != nil {
		return nil, err
	}
	ttype, err := p.parseType()
	if err != nil {
		return nil, err
	}
	var defaultValue ast.Value
	if skp, err := p.skip(lexer.EQUALS); err != nil {
		return nil, err
	} else if skp {
		dv, err := p.parseValueLiteral(true)
		if err != nil {
			return nil, err
		}
		defaultValue = dv
	}
	return &ast.VariableDefinition{
		Variable:     variable,
		Type:         ttype,
		DefaultValue: defaultValue,
		Loc:          p.loc(start),
	}, nil
}

func (p *Parser) parseVariable() (*ast.Variable, error) {
	start := p.tok.Start
	_, err := p.expect(lexer.DOLLAR)
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return &ast.Variable{
		Name: name,
		Loc:  p.loc(start),
	}, nil
}

func (p *Parser) parseSelectionSet() (*ast.SelectionSet, error) {
	start := p.tok.Start
	iSelections, err := p.many(lexer.BRACE_L, p.parseSelection, lexer.BRACE_R)
	if err != nil {
		return nil, err
	}
	selections := make([]ast.Selection, 0, len(iSelections))
	for _, iSelection := range iSelections {
		if iSelection != nil {
			// type assert interface{} into Selection interface
			selections = append(selections, iSelection.(ast.Selection))
		}
	}

	return &ast.SelectionSet{
		Selections: selections,
		Loc:        p.loc(start),
	}, nil
}

func (p *Parser) parseSelection() (interface{}, error) {
	if p.peek(lexer.SPREAD) {
		r, err := p.parseFragment()
		return r, err
	}
	return p.parseField()
}

func (p *Parser) parseField() (*ast.Field, error) {
	start := p.tok.Start
	nameOrAlias, err := p.parseName()
	if err != nil {
		return nil, err
	}
	var (
		name  *ast.Name
		alias *ast.Name
	)
	skp, err := p.skip(lexer.COLON)
	if err != nil {
		return nil, err
	} else if skp {
		alias = nameOrAlias
		name, err = p.parseName()
		if err != nil {
			return nil, err
		}
	} else {
		name = nameOrAlias
	}
	arguments, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	var selectionSet *ast.SelectionSet
	if p.peek(lexer.BRACE_L) {
		sSet, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		selectionSet = sSet
	}
	return &ast.Field{
		Alias:        alias,
		Name:         name,
		Arguments:    arguments,
		Directives:   directives,
		SelectionSet: selectionSet,
		Loc:          p.loc(start),
	}, nil
}

func (p *Parser) parseArguments() ([]*ast.Argument, error) {
	if !p.peek(lexer.PAREN_L) {
		return nil, nil
	}
	iArguments, err := p.many(lexer.PAREN_L, p.parseArgument, lexer.PAREN_R)
	if err != nil {
		return nil, err
	}
	arguments := make([]*ast.Argument, 0, len(iArguments))
	for _, iArgument := range iArguments {
		if iArgument != nil {
			arguments = append(arguments, iArgument.(*ast.Argument))
		}
	}
	return arguments, nil
}

func (p *Parser) parseArgument() (interface{}, error) {
	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	_, err = p.expect(lexer.COLON)
	if err != nil {
		return nil, err
	}
	value, err := p.parseValueLiteral(false)
	if err != nil {
		return nil, err
	}
	return &ast.Argument{
		Name:  name,
		Value: value,
		Loc:   p.loc(start),
	}, nil
}

/* Implements the parsing rules in the Fragments section. */

func (p *Parser) parseFragment() (interface{}, error) {
	start := p.tok.Start
	if _, err := p.expect(lexer.SPREAD); err != nil {
		return nil, err
	}
	if p.tok.Value == "on" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		name, err := p.parseNamed()
		if err != nil {
			return nil, err
		}
		directives, err := p.parseDirectives()
		if err != nil {
			return nil, err
		}
		selectionSet, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		return &ast.InlineFragment{
			TypeCondition: name,
			Directives:    directives,
			SelectionSet:  selectionSet,
			Loc:           p.loc(start),
		}, nil
	}
	name, err := p.parseFragmentName()
	if err != nil {
		return nil, err
	}
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	return &ast.FragmentSpread{
		Name:       name,
		Directives: directives,
		Loc:        p.loc(start),
	}, nil
}

func (p *Parser) parseFragmentDefinition() (*ast.FragmentDefinition, error) {
	start := p.tok.Start
	_, err := p.expectKeyWord("fragment")
	if err != nil {
		return nil, err
	}
	name, err := p.parseFragmentName()
	if err != nil {
		return nil, err
	}
	_, err = p.expectKeyWord("on")
	if err != nil {
		return nil, err
	}
	typeCondition, err := p.parseNamed()
	if err != nil {
		return nil, err
	}
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	selectionSet, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	return &ast.FragmentDefinition{
		Name:          name,
		TypeCondition: typeCondition,
		Directives:    directives,
		SelectionSet:  selectionSet,
		Loc:           p.loc(start),
	}, nil
}

func (p *Parser) parseFragmentName() (*ast.Name, error) {
	if p.tok.Value == "on" {
		return nil, p.unexpected(lexer.Token{})
	}
	return p.parseName()
}

/* Implements the parsing rules in the Values section. */

func (p *Parser) parseValueLiteral(isConst bool) (ast.Value, error) {
	token := p.tok
	switch token.Kind {
	case lexer.BRACKET_L:
		return p.parseList(isConst)
	case lexer.BRACE_L:
		return p.parseObject(isConst)
	case lexer.INT:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &ast.IntValue{
			Value: token.Value,
			Loc:   p.loc(token.Start),
		}, nil
	case lexer.FLOAT:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &ast.FloatValue{
			Value: token.Value,
			Loc:   p.loc(token.Start),
		}, nil
	case lexer.STRING:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &ast.StringValue{
			Value: token.Value,
			Loc:   p.loc(token.Start),
		}, nil
	case lexer.NAME:
		if token.Value == "true" || token.Value == "false" {
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &ast.BooleanValue{
				Value: token.Value == "true",
				Loc:   p.loc(token.Start),
			}, nil
		} else if token.Value != "null" {
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &ast.EnumValue{
				Value: token.Value,
				Loc:   p.loc(token.Start),
			}, nil
		}
	case lexer.DOLLAR:
		if !isConst {
			return p.parseVariable()
		}
	}
	return nil, p.unexpected(lexer.Token{})
}

func (p *Parser) parseConstValue() (interface{}, error) {
	return p.parseValueLiteral(true)
}

func (p *Parser) parseValueValue() (interface{}, error) {
	return p.parseValueLiteral(false)
}

func (p *Parser) parseList(isConst bool) (*ast.ListValue, error) {
	start := p.tok.Start
	var item parseFn
	if isConst {
		item = p.parseConstValue
	} else {
		item = p.parseValueValue
	}
	iValues, err := p.any(lexer.BRACKET_L, item, lexer.BRACKET_R)
	if err != nil {
		return nil, err
	}
	values := make([]ast.Value, len(iValues))
	for i, v := range iValues {
		values[i] = v.(ast.Value)
	}
	return &ast.ListValue{
		Values: values,
		Loc:    p.loc(start),
	}, nil
}

func (p *Parser) parseObject(isConst bool) (*ast.ObjectValue, error) {
	start := p.tok.Start
	_, err := p.expect(lexer.BRACE_L)
	if err != nil {
		return nil, err
	}
	var fields []*ast.ObjectField
	fieldNames := make(map[string]struct{})
	for {
		if skp, err := p.skip(lexer.BRACE_R); err != nil {
			return nil, err
		} else if skp {
			break
		}
		field, fieldName, err := p.parseObjectField(isConst, fieldNames)
		if err != nil {
			return nil, err
		}
		fieldNames[fieldName] = struct{}{}
		fields = append(fields, field)
	}
	return &ast.ObjectValue{
		Fields: fields,
		Loc:    p.loc(start),
	}, nil
}

func (p *Parser) parseObjectField(isConst bool, fieldNames map[string]struct{}) (*ast.ObjectField, string, error) {
	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, "", err
	}
	fieldName := name.Value
	if _, ok := fieldNames[fieldName]; ok {
		descp := fmt.Sprintf("Duplicate input object field %v.", fieldName)
		return nil, "", gqlerrors.NewSyntaxError(p.Source, start, descp)
	}
	_, err = p.expect(lexer.COLON)
	if err != nil {
		return nil, "", err
	}
	value, err := p.parseValueLiteral(isConst)
	if err != nil {
		return nil, "", err
	}
	return &ast.ObjectField{
		Name:  name,
		Value: value,
		Loc:   p.loc(start),
	}, fieldName, nil
}

/* Implements the parsing rules in the Directives section. */

func (p *Parser) parseDirectives() ([]*ast.Directive, error) {
	var directives []*ast.Directive
	for {
		if !p.peek(lexer.AT) {
			break
		}
		directive, err := p.parseDirective()
		if err != nil {
			return directives, err
		}
		directives = append(directives, directive)
	}
	return directives, nil
}

func (p *Parser) parseDirective() (*ast.Directive, error) {
	start := p.tok.Start
	_, err := p.expect(lexer.AT)
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	args, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	return &ast.Directive{
		Name:      name,
		Arguments: args,
		Loc:       p.loc(start),
	}, nil
}

/* Implements the parsing rules in the Types section. */

func (p *Parser) parseType() (ast.Type, error) {
	start := p.tok.Start
	var ttype ast.Type
	if skp, err := p.skip(lexer.BRACKET_L); err != nil {
		return nil, err
	} else if skp {
		t, err := p.parseType()
		if err != nil {
			return t, err
		}
		ttype = t
		_, err = p.expect(lexer.BRACKET_R)
		if err != nil {
			return ttype, err
		}
		ttype = &ast.List{
			Type: ttype,
			Loc:  p.loc(start),
		}
	} else {
		name, err := p.parseNamed()
		if err != nil {
			return ttype, err
		}
		ttype = name
	}
	if skp, err := p.skip(lexer.BANG); err != nil {
		return nil, err
	} else if skp {
		ttype = &ast.NonNull{
			Type: ttype,
			Loc:  p.loc(start),
		}
		return ttype, nil
	}
	return ttype, nil
}

func (p *Parser) parseNamed() (*ast.Named, error) {
	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return &ast.Named{
		Name: name,
		Loc:  p.loc(start),
	}, nil
}

/* Implements the parsing rules in the Type Definition section. */

func (p *Parser) parseObjectTypeDefinition() (*ast.ObjectDefinition, error) {
	docComment := p.leadComment

	start := p.tok.Start
	_, err := p.expectKeyWord("type")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	interfaces, err := p.parseImplementsInterfaces()
	if err != nil {
		return nil, err
	}
	iFields, err := p.any(lexer.BRACE_L, p.parseFieldDefinition, lexer.BRACE_R)
	if err != nil {
		return nil, err
	}
	fields := make([]*ast.FieldDefinition, 0, len(iFields))
	for _, iField := range iFields {
		if iField != nil {
			fields = append(fields, iField.(*ast.FieldDefinition))
		}
	}
	return &ast.ObjectDefinition{
		Name:       name,
		Loc:        p.loc(start),
		Interfaces: interfaces,
		Fields:     fields,
		Doc:        docComment,
	}, nil
}

func (p *Parser) parseImplementsInterfaces() ([]*ast.Named, error) {
	if p.tok.Value != "implements" {
		return nil, nil
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	var types []*ast.Named
	for {
		ttype, err := p.parseNamed()
		if err != nil {
			return types, err
		}
		types = append(types, ttype)
		if p.peek(lexer.BRACE_L) {
			break
		}
	}
	return types, nil
}

func (p *Parser) parseFieldDefinition() (interface{}, error) {
	docComment := p.leadComment

	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	args, err := p.parseArgumentDefs()
	if err != nil {
		return nil, err
	}
	_, err = p.expect(lexer.COLON)
	if err != nil {
		return nil, err
	}
	ttype, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ast.FieldDefinition{
		Name:      name,
		Arguments: args,
		Type:      ttype,
		Loc:       p.loc(start),
		Doc:       docComment,
		Comment:   p.lineComment,
	}, nil
}

func (p *Parser) parseArgumentDefs() ([]*ast.InputValueDefinition, error) {
	if !p.peek(lexer.PAREN_L) {
		return nil, nil
	}
	iInputValueDefinitions, err := p.many(lexer.PAREN_L, p.parseInputValueDef, lexer.PAREN_R)
	if err != nil {
		return nil, err
	}
	inputValueDefinitions := make([]*ast.InputValueDefinition, 0, len(iInputValueDefinitions))
	for _, iInputValueDefinition := range iInputValueDefinitions {
		if iInputValueDefinition != nil {
			inputValueDefinitions = append(inputValueDefinitions, iInputValueDefinition.(*ast.InputValueDefinition))
		}
	}
	return inputValueDefinitions, err
}

func (p *Parser) parseInputValueDef() (interface{}, error) {
	docComment := p.leadComment
	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	_, err = p.expect(lexer.COLON)
	if err != nil {
		return nil, err
	}
	ttype, err := p.parseType()
	if err != nil {
		return nil, err
	}
	var defaultValue ast.Value
	if skp, err := p.skip(lexer.EQUALS); err != nil {
		return nil, err
	} else if skp {
		val, err := p.parseConstValue()
		if err != nil {
			return nil, err
		}
		if val, ok := val.(ast.Value); ok {
			defaultValue = val
		}
	}
	return &ast.InputValueDefinition{
		Name:         name,
		Type:         ttype,
		DefaultValue: defaultValue,
		Loc:          p.loc(start),
		Doc:          docComment,
		Comment:      p.lineComment,
	}, nil
}

func (p *Parser) parseInterfaceTypeDefinition() (*ast.InterfaceDefinition, error) {
	docComment := p.leadComment
	start := p.tok.Start
	_, err := p.expectKeyWord("interface")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	iFields, err := p.any(lexer.BRACE_L, p.parseFieldDefinition, lexer.BRACE_R)
	if err != nil {
		return nil, err
	}
	fields := make([]*ast.FieldDefinition, 0, len(iFields))
	for _, iField := range iFields {
		if iField != nil {
			fields = append(fields, iField.(*ast.FieldDefinition))
		}
	}
	return &ast.InterfaceDefinition{
		Name:   name,
		Loc:    p.loc(start),
		Fields: fields,
		Doc:    docComment,
	}, nil
}

func (p *Parser) parseUnionTypeDefinition() (*ast.UnionDefinition, error) {
	docComment := p.leadComment
	start := p.tok.Start
	_, err := p.expectKeyWord("union")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	_, err = p.expect(lexer.EQUALS)
	if err != nil {
		return nil, err
	}
	types, err := p.parseUnionMembers()
	if err != nil {
		return nil, err
	}
	return &ast.UnionDefinition{
		Name:    name,
		Loc:     p.loc(start),
		Types:   types,
		Doc:     docComment,
		Comment: p.lineComment,
	}, nil
}

func (p *Parser) parseUnionMembers() ([]*ast.Named, error) {
	var members []*ast.Named
	for {
		member, err := p.parseNamed()
		if err != nil {
			return members, err
		}
		members = append(members, member)
		if skp, err := p.skip(lexer.PIPE); err != nil {
			return nil, err
		} else if !skp {
			break
		}
	}
	return members, nil
}

func (p *Parser) parseScalarTypeDefinition() (*ast.ScalarDefinition, error) {
	start := p.tok.Start
	_, err := p.expectKeyWord("scalar")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	def := &ast.ScalarDefinition{
		Name: name,
		Loc:  p.loc(start),
	}
	return def, nil
}

func (p *Parser) parseEnumTypeDefinition() (*ast.EnumDefinition, error) {
	docComment := p.leadComment
	start := p.tok.Start
	_, err := p.expectKeyWord("enum")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	iEnumValueDefs, err := p.any(lexer.BRACE_L, p.parseEnumValueDefinition, lexer.BRACE_R)
	if err != nil {
		return nil, err
	}
	values := make([]*ast.EnumValueDefinition, 0, len(iEnumValueDefs))
	for _, iEnumValueDef := range iEnumValueDefs {
		if iEnumValueDef != nil {
			values = append(values, iEnumValueDef.(*ast.EnumValueDefinition))
		}
	}
	return &ast.EnumDefinition{
		Name:   name,
		Loc:    p.loc(start),
		Values: values,
		Doc:    docComment,
	}, nil
}

func (p *Parser) parseEnumValueDefinition() (interface{}, error) {
	docComment := p.leadComment
	start := p.tok.Start
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return &ast.EnumValueDefinition{
		Name:    name,
		Loc:     p.loc(start),
		Doc:     docComment,
		Comment: p.lineComment,
	}, nil
}

func (p *Parser) parseInputObjectTypeDefinition() (*ast.InputObjectDefinition, error) {
	docComment := p.leadComment
	start := p.tok.Start
	_, err := p.expectKeyWord("input")
	if err != nil {
		return nil, err
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	iInputValueDefinitions, err := p.any(lexer.BRACE_L, p.parseInputValueDef, lexer.BRACE_R)
	if err != nil {
		return nil, err
	}
	fields := make([]*ast.InputValueDefinition, 0, len(iInputValueDefinitions))
	for _, iInputValueDefinition := range iInputValueDefinitions {
		if iInputValueDefinition != nil {
			fields = append(fields, iInputValueDefinition.(*ast.InputValueDefinition))
		}
	}
	return &ast.InputObjectDefinition{
		Name:   name,
		Loc:    p.loc(start),
		Fields: fields,
		Doc:    docComment,
	}, nil
}

func (p *Parser) parseTypeExtensionDefinition() (*ast.TypeExtensionDefinition, error) {
	start := p.tok.Start
	_, err := p.expectKeyWord("extend")
	if err != nil {
		return nil, err
	}

	definition, err := p.parseObjectTypeDefinition()
	if err != nil {
		return nil, err
	}
	return &ast.TypeExtensionDefinition{
		Loc:        p.loc(start),
		Definition: definition,
	}, nil
}

/* Core parsing utility functions */

// Returns a location object, used to identify the place in
// the source that created a given parsed object.
func (p *Parser) loc(start int) ast.Location {
	if p.Options.NoSource {
		return ast.Location{
			Start: start,
			End:   p.prevEnd,
		}
	}
	return ast.Location{
		Start:  start,
		End:    p.prevEnd,
		Source: p.Source,
	}
}

// advance moves the internal parser object to the next lexed token.
func (p *Parser) advance() error {
	p.prevEnd = p.tok.End
	return p.next()
}

func (p *Parser) next0() error {
	var err error
	p.tok, err = p.Lexer.NextToken()
	return err
}

// next returns the next token from the lexer skipping over comments
func (p *Parser) next() error {
	if !p.Options.KeepComments {
		for {
			if err := p.next0(); err != nil {
				return err
			}
			if p.tok.Kind != lexer.COMMENT {
				return nil
			}
		}
	}

	p.leadComment = nil
	p.lineComment = nil
	prev := p.tok.Start
	if err := p.next0(); err != nil {
		return err
	}

	if p.tok.Kind == lexer.COMMENT {
		var comment *ast.CommentGroup
		var endline int
		var err error

		if p.posToLine(p.tok.Start) == p.posToLine(prev) {
			// The comment is on same line as the previous token; it
			// cannot be a lead comment but may be a line comment.
			comment, endline, err = p.consumeCommentGroup(0)
			if err != nil {
				return err
			}
			if p.posToLine(p.tok.Start) != endline {
				// The next token is on a different line, thus
				// the last comment group is a line comment.
				p.lineComment = comment
			}
		}

		// consume successor comments, if any
		endline = -1
		for p.tok.Kind == lexer.COMMENT {
			comment, endline, err = p.consumeCommentGroup(1)
			if err != nil {
				return err
			}
		}

		if endline+1 == p.posToLine(p.tok.Start) {
			// The next token is following on the line immediately after the
			// comment group, thus the last comment group is a lead comment.
			p.leadComment = comment
		}
	}

	return nil
}

func (p *Parser) posToLine(pos int) int {
	return p.Source.Position(pos).Line
}

func (p *Parser) consumeCommentGroup(n int) (comments *ast.CommentGroup, endline int, err error) {
	var list []*ast.Comment
	endline = p.posToLine(p.tok.Start)
	for p.tok.Kind == lexer.COMMENT && p.posToLine(p.tok.Start) <= endline+n {
		endline = p.posToLine(p.tok.Start)
		comment := &ast.Comment{
			Loc:  ast.Location{Start: p.tok.Start, End: p.tok.End},
			Text: p.tok.Value,
		}
		if !p.Options.NoSource {
			comment.Loc.Source = p.Source
		}
		list = append(list, comment)
		if err := p.next0(); err != nil {
			return nil, 0, err
		}
	}

	// add comment group to the comments list
	comments = &ast.CommentGroup{
		Loc:  ast.Location{Start: list[0].Loc.Start, End: list[len(list)-1].Loc.End, Source: list[0].Loc.Source},
		List: list,
	}
	p.comments = append(p.comments, comments)

	return
}

// Determines if the next token is of a given kind
func (p *Parser) peek(Kind int) bool {
	return p.tok.Kind == Kind
}

// If the next token is of the given kind, return true after advancing
// the parser. Otherwise, do not change the parser state and return false.
func (p *Parser) skip(Kind int) (bool, error) {
	if p.tok.Kind == Kind {
		return true, p.advance()
	}
	return false, nil
}

// If the next token is of the given kind, return that token after advancing
// the parser. Otherwise, do not change the parser state and return error.
func (p *Parser) expect(kind int) (lexer.Token, error) {
	token := p.tok
	if token.Kind == kind {
		return token, p.advance()
	}
	descp := fmt.Sprintf("Expected %s, found %s", lexer.GetTokenKindDesc(kind), lexer.GetTokenDesc(token))
	return token, gqlerrors.NewSyntaxError(p.Source, token.Start, descp)
}

// If the next token is a keyword with the given value, return that token after
// advancing the parser. Otherwise, do not change the parser state and return false.
func (p *Parser) expectKeyWord(value string) (lexer.Token, error) {
	token := p.tok
	if token.Kind == lexer.NAME && token.Value == value {
		return token, p.advance()
	}
	descp := fmt.Sprintf("Expected \"%s\", found %s", value, lexer.GetTokenDesc(token))
	return token, gqlerrors.NewSyntaxError(p.Source, token.Start, descp)
}

// Helper function for creating an error when an unexpected lexed token
// is encountered.
func (p *Parser) unexpected(atToken lexer.Token) error {
	token := atToken
	if (token == lexer.Token{}) {
		token = p.tok
	}
	description := fmt.Sprintf("Unexpected %v", lexer.GetTokenDesc(token))
	return gqlerrors.NewSyntaxError(p.Source, token.Start, description)
}

// any returns a possibly empty list of parse nodes, determined by
// the parseFn. This list begins with a lex token of openKind
// and ends with a lex token of closeKind. Advances the parser
// to the next lex token after the closing token.
func (p *Parser) any(openKind int, parseFn parseFn, closeKind int) ([]interface{}, error) {
	if _, err := p.expect(openKind); err != nil {
		return nil, err
	}
	var nodes []interface{}
	for {
		if skp, err := p.skip(closeKind); err != nil {
			return nil, err
		} else if skp {
			break
		}
		n, err := parseFn()
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// many returns a non-empty list of parse nodes, determined by
// the parseFn. This list begins with a lex token of openKind
// and ends with a lex token of closeKind. Advances the parser
// to the next lex token after the closing token.
func (p *Parser) many(openKind int, parseFn parseFn, closeKind int) ([]interface{}, error) {
	_, err := p.expect(openKind)
	if err != nil {
		return nil, err
	}
	node, err := parseFn()
	if err != nil {
		return nil, err
	}
	var nodes []interface{}
	nodes = append(nodes, node)
	for {
		if skp, err := p.skip(closeKind); err != nil {
			return nil, err
		} else if skp {
			break
		}
		node, err := parseFn()
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
