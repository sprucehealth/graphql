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
	g.printf("\tpath string\n")
	g.printf("\tauthToken string\n")
	g.printf("}\n")
	g.printf("func New(endpoint, path, authToken string) Client {\n")
	g.printf("\treturn &client{\n")
	g.printf("\t\tpath: path,\n")
	g.printf("\t\tendpoint: endpoint,\n")
	g.printf("\t\tauthToken: authToken,\n")
	g.printf("\t}\n")
	g.printf("}\n")
	g.printf("\n")
	for _, def := range clientTypeDefs {
		for _, field := range def.Fields {
			genClientMethodForField(g, def, field)
			g.printf("\n")
		}
	}
	genDecoderTypeMap(g)
	g.printf("\n")
	genDecoderHook(g)
	g.printf("\n")
	genQueryWrapperTypes(g)
	g.printf("\n")
	genMakeURL(g)
	g.printf("\n")
	genClientDo(g)
	g.printf("\n")
	genRewriteQuery(g)
	g.printf("\n")
}

func renderSignatureForField(g *generator, d *ast.ObjectDefinition, f *ast.FieldDefinition) string {
	return fmt.Sprintf("%s%s(ctx context.Context, query string) (%s, error)", exportedName(d.Name.Value), exportedName(f.Name.Value), g.goType(f.Type, ""))
}

func genDecoderHook(g *generator) {
	g.printf("func decoderHook(from reflect.Kind, to reflect.Kind, v interface{}) (interface{}, error) {\n")
	g.printf("\tif from == reflect.Map && to == reflect.Interface {\n")
	g.printf("\t\tcVal := reflect.New(objectTypesByTypename[v.(map[string]interface{})[\"__typename\"].(string)])\n")
	g.printf("\t\treturn cVal.Elem().Addr().Interface(), nil\n")
	g.printf("\t}\n")
	g.printf("\t\treturn v, nil\n")
	g.printf("}\n")
}

func genDecoderTypeMap(g *generator) {
	g.printf("var objectTypesByTypename = map[string]reflect.Type{\n")
	for _, def := range g.doc.Definitions {
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			g.printf("\"%s\": reflect.TypeOf(%s{}),\n", def.Name.Value, def.Name.Value)
		}
	}
	g.printf("}\n")
}

func genOutputTypeVar(g *generator, f *ast.FieldDefinition) {
	outType := g.goType(f.Type, "")
	if outType[0] == '*' {
		outType = outType[1:]
	}
	g.printf("\tvar out %s\n", outType)
}

func outputTypeReturn(g *generator, f *ast.FieldDefinition) string {
	outType := g.goType(f.Type, "")
	if outType[0] == '*' {
		return "&out"
	}
	return "out"
}

func genClientMethodForField(g *generator, d *ast.ObjectDefinition, f *ast.FieldDefinition) {
	g.printf("func (c *client) %s {\n", renderSignatureForField(g, d, f))
	g.printf("\tgolog.ContextLogger(ctx).Debugf(\"%s%s\")\n", exportedName(d.Name.Value), exportedName(f.Name.Value))
	genOutputTypeVar(g, f)
	g.printf("\tif _, err := c.do(ctx, \"%s\", query, &out); err != nil {\n", unexportedName(f.Name.Value))
	g.printf("\t\treturn %s, errors.Trace(err)\n", renderDefaultReturnValue(f.Type))
	g.printf("\t}\n")
	g.printf("\treturn %s, nil", outputTypeReturn(g, f))
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

func genMakeURL(g *generator) {
	g.printf("func (c *client) makeURL() string {\n")
	g.printf("\tscheme := \"https\"\n")
	g.printf("\tif environment.IsLocal() {\n")
	g.printf("\t\t scheme=\"http\"\n")
	g.printf("}\n")
	g.printf("\turl := &url.URL{\n")
	g.printf("\tScheme: scheme,\n")
	g.printf("\t\tHost:   c.endpoint,\n")
	g.printf("\t\tPath:   c.path,\n")
	g.printf("\t}\n")
	g.printf("\treturn url.String()\n")
	g.printf("}\n")
}

func genQueryWrapperTypes(g *generator) {
	g.printf("type gqlRequestBody struct{\n")
	g.printf("\tQuery string `json:\"query\"`\n")
	g.printf("}\n")
	g.printf("\n")
	g.printf("type gqlResponse struct{\n")
	g.printf("\tData map[string]interface{} `json:\"data\"`\n")
	g.printf("\tErrors []map[string]interface{} `json:\"errors\"`\n")
	g.printf("}\n")
}

func genRewriteQuery(g *generator) {
	g.printf("func rewriteQuery(query string) (string, error) {\n")
	g.printf("\tqast, err := parser.Parse(parser.ParseParams{\n")
	g.printf("\t\tSource: source.New(\"GraphQL Query\", query),\n")
	g.printf("\t})\n")
	g.printf("\tif err != nil {\n")
	g.printf("\t\treturn \"\", errors.Trace(err)\n")
	g.printf("\t}\n")
	g.printf("\tfor _, node := range qast.Definitions {\n")
	g.printf("\t\tswitch od := node.(type) {\n")
	g.printf("\t\tcase *ast.OperationDefinition:\n")
	g.printf("\t\t\trewriteSelectionSet(od.SelectionSet)\n")
	g.printf("\t\t}\n")
	g.printf("\t}\n")
	g.printf("\treturn printer.Print(qast), nil\n")
	g.printf("}\n")
	g.printf("\n")
	g.printf("func rewriteSelectionSet(ss *ast.SelectionSet) error {\n")
	g.printf("\tif ss == nil {\n")
	g.printf("\t\treturn nil\n")
	g.printf("\t}\n")
	g.printf("\tss.Selections = append(ss.Selections, &ast.Field{\n")
	g.printf("\t\tName: &ast.Name{\n")
	g.printf("\t\t\tValue: \"__typename\",\n")
	g.printf("\t\t},\n")
	g.printf("\t})\n")
	g.printf("\tfor _, selections := range ss.Selections {\n")
	g.printf("\t\tif err := rewriteSelectionSet(selections.GetSelectionSet()); err != nil {\n")
	g.printf("\t\t\treturn errors.Trace(err)\n")
	g.printf("\t\t}\n")
	g.printf("\t}\n")
	g.printf("\treturn nil\n")
	g.printf("}\n")
}

func genClientDo(g *generator) {
	g.printf("func (c *client) do(ctx context.Context, dataField, query string, out interface{}) (int, error){\n")
	g.printf("\tquery, err := rewriteQuery(query)\n")
	g.printf("\tif err != nil {\n")
	g.printf("\t\treturn 0, errors.Trace(err)\n")
	g.printf("\t}\n")
	g.print("\trb := &gqlRequestBody{Query: query}\n")
	g.printf("\tbBody, err := json.Marshal(rb)\n")
	g.printf("\tif err != nil {\n")
	g.printf("\t\treturn 0, errors.Trace(err)\n")
	g.printf("\t}\n")
	g.printf("\tu := c.makeURL()\n")
	g.print("\tgolog.ContextLogger(ctx).Debugf(\"Request: %s - %s\", u, string(bBody))\n")
	g.printf("\treq, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(bBody))\n")
	g.printf("\tif err != nil {\n")
	g.printf("\t\treturn 0, errors.Trace(err)\n")
	g.printf("\t}\n")
	g.printf("\tif c.authToken != \"\" {\n")
	g.printf("\t\treq.AddCookie(&http.Cookie{\n")
	g.printf("\t\t\tName:     \"at\",\n")
	g.printf("\t\t\tDomain:   c.endpoint,\n")
	g.printf("\t\t\tValue:    c.authToken,\n")
	g.printf("\t\t\tPath:     \"/\",\n")
	g.printf("\t\t\tSecure:   !(environment.IsDev() || environment.IsLocal()),\n")
	g.printf("\t\t\tHttpOnly: true,\n")
	g.printf("\t\t})\n")
	g.printf("\t}\n")
	g.printf("\tresp, err := http.DefaultClient.Do(req)\n")
	g.printf("\tif err != nil {\n")
	g.printf("\t\treturn 0, errors.Trace(err)\n")
	g.printf("\t}\n")
	g.printf("\tball, err := ioutil.ReadAll(resp.Body)\n")
	g.printf("\tif err != nil {\n")
	g.print("\t\treturn resp.StatusCode, errors.Wrapf(err, \"Error reading body - in response from %v\", req.URL.String())\n")
	g.printf("\t}\n")
	g.print("\tgolog.ContextLogger(ctx).Debugf(\"Response: %s - %s\", resp.Status, string(ball))\n")
	g.printf("\tgqlResp := &gqlResponse{}\n")
	g.printf("\tif resp.StatusCode == http.StatusOK {\n")
	g.printf("\t\tif err := json.NewDecoder(bytes.NewReader(ball)).Decode(gqlResp); err != nil {\n")
	g.printf("\t\t\treturn resp.StatusCode, errors.Wrapf(err, \"Error parsing body into output\")\n")
	g.printf("\t\t}\n")
	g.printf("\t\tif len(gqlResp.Errors) != 0 {\n")
	g.printf("\t\t\tvar allErrors string\n")
	g.printf("\t\t\tfor i, err := range gqlResp.Errors {\n")
	g.print("\t\t\t\tallErrors += fmt.Sprintf(\"%d. - %+v\\n\", i, err)\n")
	g.printf("\t\t\t}\n")
	g.printf("\t\t\treturn resp.StatusCode, errors.Errorf(allErrors)\n")
	g.printf("\t\t}\n")
	g.printf("\t\tdecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{\n")
	g.printf("\t\t\tDecodeHook: decoderHook,\n")
	g.printf("\t\t\tResult:     out,\n")
	g.printf("\t\t})\n")
	g.printf("\t\tif err != nil {\n")
	g.printf("\t\t\treturn 0, errors.Trace(err)\n")
	g.printf("\t\t}\n")
	g.printf("\t\tif err := decoder.Decode(gqlResp.Data[dataField].(map[string]interface{})); err != nil {\n")
	g.printf("\t\t\treturn resp.StatusCode, errors.Wrapf(err, \"Error parsing body into output\")\n")
	g.printf("\t\t}\n")
	g.printf("\t\treturn resp.StatusCode, nil\n")
	g.printf("\t}\n")
	g.print("\treturn resp.StatusCode, errors.Errorf(\"Non 200 Response (%d) from %s: %s\", resp.StatusCode, req.URL, string(ball))\n")
	g.printf("}\n")
}
