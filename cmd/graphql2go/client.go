package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sprucehealth/graphql/language/ast"
)

func generateClient(g *generator) {
	imports := []string{"github.com/sprucehealth/graphql"}
	if len(g.cfg.Resolvers) != 0 {
		imports = []string{
			"context",
			"fmt",
			"reflect",
			"",
			"github.com/sprucehealth/graphql",
			"github.com/sprucehealth/graphql/gqldecode",
			"github.com/sprucehealth/graphql/gqlerrors",
			"github.com/sprucehealth/graphql/language/parser",
			"github.com/sprucehealth/graphql/language/printer",
			"github.com/sprucehealth/graphql/language/source",
			"github.com/sprucehealth/mapstructure",
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
	g.print(`
		type client struct {
			endpoint string
			authToken string
			log Logger
		}

		type clientOption func(c *client)

		func WithClientLogger(l Logger) clientOption {
			return func(c *client) {
				c.log = l
			}
		}

		type Logger interface {
			Debugf(ctx context.Context, msg string, v ...interface{})
		}

		type nullLogger struct{}
		func (nullLogger) Debugf(ctx context.Context, msg string, v ...interface{}){}

		func New(endpoint, authToken string, opts ...clientOption) Client {
			c := &client{
				endpoint: endpoint,
				authToken: authToken,
				log: nullLogger{},
			}
			for _, o := range opts {
				o(c)
			}
			return c
		}
	`)
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
	g.printf("\t\tout := cVal.Elem().Addr().Interface()\n")
	g.printf("\t\tdecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{\n")
	g.printf("\t\t\tDecodeHook: decoderHook,\n")
	g.printf("\t\t\tResult:     out,\n")
	g.printf("\t\t})\n")
	g.printf("\t\tif err != nil {\n")
	g.printf("\t\t\treturn 0, err\n")
	g.printf("\t\t}\n")
	g.printf("\t\tif err := decoder.Decode(v.(map[string]interface{})); err != nil {\n")
	g.print("\t\t\treturn nil, fmt.Errorf(\"Error decoding %+v into %+v\", v, out)\n")
	g.printf("\t\t}\n")
	g.printf("\t\treturn out, nil\n")
	g.printf("\t}\n")
	g.printf("\treturn v, nil\n")
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
	g.printf("\tc.log.Debugf(ctx, \"%s%s\")\n", exportedName(d.Name.Value), exportedName(f.Name.Value))
	genOutputTypeVar(g, f)
	g.printf("\tif _, err := c.do(ctx, \"%s\", query, &out); err != nil {\n", unexportedName(f.Name.Value))
	g.printf("\t\treturn %s, err\n", renderDefaultReturnValue(f.Type))
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

func genQueryWrapperTypes(g *generator) {
	g.print(`
		type gqlRequestBody struct {
			Query string ` + "`json:\"query\"`" + `
		}

		type gqlResponse struct {
			Data   map[string]interface{}   ` + "`json:\"data\"`" + `
			Errors []map[string]interface{} ` + "`json:\"errors\"`" + `
		}
	`)
}

func genRewriteQuery(g *generator) {
	g.print(`
		func rewriteQuery(query string) (string, error) {
			qast, err := parser.Parse(parser.ParseParams{
				Source: source.New("GraphQL Query", query),
			})
			if err != nil {
				return "", err
			}
			graphql.RequestTypeNames(qast)
			return printer.Print(qast), nil
		}
	`)
}

func genClientDo(g *generator) {
	g.print(`
		func (c *client) do(ctx context.Context, dataField, query string, out interface{}) (int, error) {
			query, err := rewriteQuery(query)
			if err != nil {
				return 0, err
			}
			rb := &gqlRequestBody{Query: query}
			bBody, err := json.Marshal(rb)
			if err != nil {
				return 0, err
			}
			c.log.Debugf(ctx, "Request: %s - %s", c.endpoint, string(bBody))
			req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(bBody))
			if err != nil {
				return 0, err
			}
			if c.authToken != "" {
				req.AddCookie(&http.Cookie{
					Name:     "at",
					Value:    c.authToken,
				})
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return 0, err
			}
			ball, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return resp.StatusCode, fmt.Errorf("Error reading body - in response from %v: %s", req.URL.String(), err)
			}
			c.log.Debugf(ctx, "Response: %s - %s", resp.Status, ball)
			gqlResp := &gqlResponse{}
			if resp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(bytes.NewReader(ball)).Decode(gqlResp); err != nil {
					return resp.StatusCode, fmt.Errorf("Error parsing body into output: %s", err)
				}
				if len(gqlResp.Errors) != 0 {
					var allErrors string
					for i, err := range gqlResp.Errors {
						allErrors += fmt.Sprintf("%d. - %+v\n", i, err)
					}
					return resp.StatusCode, errors.New(allErrors)
				}
				decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
					DecodeHook: decoderHook,
					Result:     out,
				})
				if err != nil {
					return 0, err
				}
				mData, ok := gqlResp.Data[dataField].(map[string]interface{})
				if ok {
					if err := decoder.Decode(mData); err != nil {
						return resp.StatusCode, fmt.Errorf("Error parsing body into output: %s", err)
					}
				} else {
					sData, ok := gqlResp.Data[dataField].([]interface{})
					if ok {
						if err := decoder.Decode(sData); err != nil {
							return resp.StatusCode, fmt.Errorf("Error parsing body into output: %s", err)
						}
					} else {
						return resp.StatusCode, fmt.Errorf("unhandled response data type %T %+v", gqlResp.Data[dataField])
					}
				}
				
				return resp.StatusCode, nil
			}
			return resp.StatusCode, fmt.Errorf("Non 200 Response (%d) from %s: %s", resp.StatusCode, req.URL, string(ball))
		}
	`)
}
