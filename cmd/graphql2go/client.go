package main

import (
	"strings"

	"sort"

	"fmt"

	"github.com/sprucehealth/graphql/language/ast"
)

func generateClient(g *generator) {
	imports := []string{"github.com/sprucehealth/graphql"}
	if len(g.cfg.Resolvers) != 0 {
		imports = []string{
			"context",
			"fmt",
			"",
			"github.com/sprucehealth/backend/libs/gqldecode",
			"github.com/sprucehealth/graphql",
			"github.com/sprucehealth/graphql/gqlerrors",
		}
	}

	g.printf("package client\n\n")
	g.printf("import (\n")
	for _, im := range imports {
		if im == "" {
			g.printf("\n")
		} else {
			g.printf("\t%q\n", im)
		}
	}
	g.printf(")\n\n")

	clientTypes := make(map[string]struct{})
	for _, t := range strings.Split(*flagClientTypes, ",") {
		clientTypes[t] = struct{}{}
	}

	var clientTypeDefs []*ast.ObjectDefinition
	for _, def := range g.doc.Definitions {
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			if _, ok := clientTypes[def.Name.Value]; ok {
				clientTypeDefs = append(clientTypeDefs, def)
			}
			g.genObjectModel(def)
			g.printf("\n")
		case *ast.InterfaceDefinition:
			g.genInterfaceModel(def)
			g.printf("\n")
		case *ast.InputObjectDefinition:
			g.genInputModel(def)
			g.printf("\n")
		case *ast.UnionDefinition:
			g.genUnionModel(def)
			g.printf("\n")
		case *ast.EnumDefinition:
			g.genEnumConstants(def)
			g.printf("\n")
		}
	}

	var sigs []string
	for _, def := range clientTypeDefs {
		for _, field := range def.Fields {
			sigs = append(sigs, renderSignatureForField(g, def, field))
		}
	}
	sort.Strings(sigs)
	g.printf("type Client interface {\n")
	for _, sig := range sigs {
		g.printf(sig + "\n")
	}
	g.printf("}\n")
	g.printf("\n")
	g.printf("type client struct {\n")
	g.printf("\tendpoint string\n")
	g.printf("}\n")
	g.printf("func New(endpoint string) Client {\n")
	g.printf("\treturn &client{endpoint: endpoint}\n")
	g.printf("}\n")
	g.printf("\n")
	for _, def := range clientTypeDefs {
		for _, field := range def.Fields {
			genClientMethodForField(g, def, field)
			g.printf("\n")
		}
	}
}

func renderSignatureForField(g *generator, d *ast.ObjectDefinition, f *ast.FieldDefinition) string {
	var argType string
	if len(f.Arguments) != 0 {
		argType = fmt.Sprintf("*%s%sArgs", exportedName(d.Name.Value), exportedName(f.Name.Value))
	}
	return fmt.Sprintf("%s%s(%s) (%s, error)", exportedName(d.Name.Value), exportedName(f.Name.Value), argType, g.goType(f.Type, ""))
}

func genClientMethodForField(g *generator, d *ast.ObjectDefinition, f *ast.FieldDefinition) {
	g.printf("func (c *client) %s {\n", renderSignatureForField(g, d, f))
	g.printf("\treturn %s, nil", renderDefaultReturnValue(f.Type))
	g.printf("}\n")
}

func renderDefaultClientReturnValue(t ast.Type) string {
	switch t := t.(type) {
	case *ast.NonNull:
		return renderDefaultClientReturnValue(t.Type)
	case *ast.Named:
		switch t.Name.Value {
		case "ID", "String":
			return `""`
		case "Boolean":
			return "false"
		case "Float", "Int":
			return "0"
		}
	}
	return "nil"
}
