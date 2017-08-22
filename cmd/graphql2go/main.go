package main

// TODO: default values for input fields and arguments

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/parser"
)

var (
	flagArtifact       = flag.String("artifact", "server", "The artifact to generate from the schema (server or client)")
	flagClientTypes    = flag.String("client_types", "Query,Mutation", "The types that should be used to create client methods")
	flagConfigFile     = flag.String("config", "", "Path to config file")
	flagOutFile        = flag.String("out", "", "Path to output file (stdout if not set)")
	flagSchemaFile     = flag.String("schema", "", "Path to schema file (stdin if not set)")
	flagNullableInputs = flag.Bool("nullable_inputs", false, "Flag to determine if nullable inputs should be serialized into pointers")
	flagVerbose        = flag.Bool("v", false, "Verbose output")
)

var initialisms = map[string]string{
	"CTA":   "CTA",
	"DOB":   "DOB",
	"HTTP":  "HTTP",
	"HTTPS": "HTTPS",
	"EMR":   "EMR",
	"HMAC":  "HMAC",
	"ID":    "ID",
	"IDS":   "IDs",
	"IOS":   "IOS",
	"LAN":   "LAN",
	"OTC":   "OTC",
	"SAML":  "SAML",
	"SMS":   "SMS",
	"UID":   "UID",
	"URL":   "URL",
	"UUID":  "UUID",
	"VOIP":  "VOIP",
}

type config struct {
	Resolvers        map[string][]string          // type -> fields
	CustomFieldTypes map[string]string            // Type.Field -> go type
	ExtraFields      map[string]map[string]string // type -> field -> go type
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	var schema []byte
	if *flagSchemaFile != "" {
		var err error
		schema, err = ioutil.ReadFile(*flagSchemaFile)
		if err != nil {
			log.Fatalf("Failed to read schema file %q: %s", *flagSchemaFile, err)
		}
	} else {
		var err error
		schema, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Failed to read schema from stdin: %s", err)
		}
	}
	root, err := parser.Parse(parser.ParseParams{
		Source: string(schema),
		Options: parser.ParseOptions{
			NoSource:     false,
			KeepComments: true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	var outWriter io.Writer
	if *flagOutFile == "" {
		outWriter = os.Stdout
	} else {
		fo, err := os.Create(*flagOutFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %s", err)
		}
		defer fo.Close()
		outWriter = fo
	}

	g := newGenerator(outWriter, root)

	switch *flagArtifact {
	case "server":
		generateServer(g)
	case "client":
		generateClient(g)
	default:
		log.Fatalf("Unknown output artifact type %s", *flagArtifact)
	}
}

type resolver struct {
	typeName string
	fields   []string
}

func generateServer(g *generator) {
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

	g.printf("package schema\n\n")
	g.printf("import (\n")
	for _, im := range imports {
		if im == "" {
			g.printf("\n")
		} else {
			g.printf("\t%q\n", im)
		}
	}
	g.printf(")\n\n")

	// Turn the resolver map into a slice to be able to sort and have consistent order
	resolvers := make([]*resolver, 0, len(g.cfg.Resolvers))
	for typeName, fields := range g.cfg.Resolvers {
		resolvers = append(resolvers, &resolver{typeName: typeName, fields: fields})
	}
	sort.Slice(resolvers, func(i, j int) bool { return resolvers[i].typeName < resolvers[j].typeName })

	// Validate custom resolvers and generate interfaces
	for _, r := range resolvers {
		typeName := r.typeName
		fields := r.fields
		assertionType := fmt.Sprintf("*%s", exportedName(typeName))
		if isTopLevelObject(exportedName(typeName)) {
			assertionType = "map[string]interface{}"
		}
		sort.Strings(fields)
		g.printf("const %sResolversKey = %q\n\n", exportedName(typeName), exportedName(typeName)+"Resolvers")
		g.printf("type %sResolvers interface {\n", exportedName(typeName))
		for _, fieldName := range fields {
			objDef, ok := g.types[typeName].(*ast.ObjectDefinition)
			if !ok || objDef == nil {
				log.Fatalf("Unknown object definition %q when generating resolvers", typeName)
			}
			var field *ast.FieldDefinition
			for _, f := range objDef.Fields {
				if f.Name.Value == fieldName {
					field = f
					break
				}
			}
			if field == nil {
				log.Fatalf("Unknown field %q on object %q when generating resolvers", fieldName, typeName)
			}
			if len(field.Arguments) == 0 {
				g.printf("\t%s(ctx context.Context, parent %s, p graphql.ResolveParams) (%s, error)\n",
					exportedName(field.Name.Value), assertionType, g.goType(field.Type, objDef.Name.Value+"."+field.Name.Value))
			} else {
				g.printf("\t%s(ctx context.Context, parent %s, args *%s%sArgs, p graphql.ResolveParams) (%s, error)\n",
					exportedName(field.Name.Value), assertionType, exportedName(objDef.Name.Value), exportedName(field.Name.Value), g.goType(field.Type, objDef.Name.Value+"."+field.Name.Value))
			}
		}
		g.printf("}\n\n")
	}
	// Generate types
	for _, def := range g.doc.Definitions {
		g.genNode(def)
	}
	// Generate a list of all the types
	g.printf("\nvar TypeDefs = []graphql.Type{\n")
	for _, def := range g.doc.Definitions {
		var name string
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			name = goObjectDefName(def.Name.Value)
		case *ast.InterfaceDefinition:
			name = goInterfaceDefName(def.Name.Value)
		case *ast.UnionDefinition:
			name = goUnionDefName(def.Name.Value)
		case *ast.InputObjectDefinition:
			name = goInputObjectDefName(def.Name.Value)
		case *ast.EnumDefinition:
			name = goEnumDefName(def.Name.Value)
		default:
			log.Fatalf("Unhandled node type %T", def)
		}
		g.printf("\t%s,\n", name)
	}
	g.printf("}\n")
}

func newGenerator(outWriter io.Writer, root *ast.Document) *generator {
	g := &generator{
		w:            outWriter,
		doc:          root,
		types:        make(map[string]ast.Node),
		cycles:       make(map[string][]string),
		typeUseCount: make(map[string]int),
		cycleBreaks:  make(map[string]map[string]struct{}),
	}
	if *flagConfigFile != "" {
		b, err := ioutil.ReadFile(*flagConfigFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %s", err)
		}
		if err := json.Unmarshal(b, &g.cfg); err != nil {
			log.Fatalf("Failed to decode config file: %s", err)
		}
	}

	// Generate index of type name to definition and make sure all names are unique
	for _, def := range root.Definitions {
		var name string
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			name = def.Name.Value
		case *ast.InputObjectDefinition:
			name = def.Name.Value
		case *ast.EnumDefinition:
			name = def.Name.Value
		case *ast.InterfaceDefinition:
			name = def.Name.Value
		case *ast.UnionDefinition:
			name = def.Name.Value
		default:
			log.Fatalf("Unhandled node type %T", def)
		}
		if _, ok := g.types[name]; ok {
			log.Fatalf("Duplicate type name %q", name)
		}
		g.types[name] = def
	}

	// Detect cycles in types
	for _, def := range root.Definitions {
		g.findCycles(def, nil)
	}

	// Pick the least used object type in each cycle to use as the broken link
	for _, path := range g.cycles {
		var name string
		var minCount int
		pathMap := make(map[string]struct{}, len(path))
		for _, n := range path {
			pathMap[n] = struct{}{}
			node := g.types[n]
			if _, ok := node.(*ast.ObjectDefinition); !ok {
				continue
			}
			if c := g.typeUseCount[n]; minCount == 0 || c < minCount || (c == minCount && n < name) {
				minCount = c
				name = n
			}
		}
		// Merge if necessary
		if cb := g.cycleBreaks[name]; cb != nil {
			for n := range pathMap {
				cb[n] = struct{}{}
			}
		} else {
			g.cycleBreaks[name] = pathMap
		}
		log.Printf("Cycle: %s [breaking with %s]\n", strings.Join(path, " → "), name)
	}

	if g.cfg.Resolvers == nil {
		g.cfg.Resolvers = make(map[string][]string)
	}

	// Look for top level types to enforce resolvers
	for _, def := range root.Definitions {
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			if isTopLevelObject(def.Name.Value) {
				fieldNames := make([]string, len(def.Fields))
				for i, f := range def.Fields {
					fieldNames[i] = f.Name.Value
				}
				g.cfg.Resolvers[exportedName(def.Name.Value)] = fieldNames
			}
		}
	}
	return g
}

func cycleKey(path []string) string {
	// Avoid modifying the path so clone it
	p := append([]string(nil), path...)
	sort.Strings(p)
	return strings.Join(p, "→")
}

type generator struct {
	w            io.Writer
	cfg          config
	doc          *ast.Document
	types        map[string]ast.Node
	cycles       map[string][]string
	typeUseCount map[string]int
	cycleBreaks  map[string]map[string]struct{} // names of types to break cycles (least used type in a cycle) → types for fields to use placeholders
}

func stringsIndex(sl []string, s string) int {
	for i, x := range sl {
		if x == s {
			return i
		}
	}
	return -1
}

func (g *generator) findCycles(def ast.Node, ancestors []string) {
	var name string
	var types []ast.Type

	switch def := def.(type) {
	case *ast.ObjectDefinition:
		name = def.Name.Value
		types = make([]ast.Type, len(def.Fields))
		for i, f := range def.Fields {
			types[i] = f.Type
		}
	case *ast.InterfaceDefinition:
		name = def.Name.Value
		types = make([]ast.Type, len(def.Fields))
		for i, f := range def.Fields {
			types[i] = f.Type
		}
	case *ast.UnionDefinition:
		name = def.Name.Value
		types = make([]ast.Type, len(def.Types))
		for i, t := range def.Types {
			types[i] = t
		}
	case *ast.InputObjectDefinition, *ast.EnumDefinition:
		// Input objects and enums can't form cycles
		return
	default:
		log.Fatalf("Unhandled node type %T", def)
	}

	g.typeUseCount[name]++

	if i := stringsIndex(ancestors, name); i >= 0 {
		// Clone the path into a new slice
		path := append([]string(nil), ancestors[i:]...)
		g.cycles[cycleKey(path)] = path
		return
	}
	ancestors = append(ancestors, name)
	for _, typ := range types {
		t := g.defForType(typ)
		if t == nil {
			g.failf("Could not resolve type %T %s", typ, typ)
		}
		if _, ok := t.(*ast.Named); !ok {
			g.findCycles(t, ancestors)
		}
	}
}

// defForType recursively walks a type until it gets to the definition type.
// If the type is a base type (e.g. String) then it returns ast.NamedType.
// It returns nil if the type can't be resolved.
func (g *generator) defForType(t ast.Type) ast.Node {
	switch t := t.(type) {
	case *ast.NonNull:
		return g.defForType(t.Type)
	case *ast.List:
		return g.defForType(t.Type)
	case *ast.Named:
		switch t.Name.Value {
		case "ID", "String", "Boolean", "Float", "Int":
			return t
		}
		return g.types[t.Name.Value]
	}
	log.Fatalf("Unhandled type %T", t)
	return nil
}

func (g *generator) baseTypeName(t ast.Type) string {
	switch t := t.(type) {
	case *ast.NonNull:
		return g.baseTypeName(t.Type)
	case *ast.List:
		return g.baseTypeName(t.Type)
	case *ast.Named:
		return t.Name.Value
	}
	log.Fatalf("Unhandled type %T", t)
	return ""
}

func (g *generator) printf(m string, a ...interface{}) {
	if _, err := fmt.Fprintf(g.w, m, a...); err != nil {
		g.fail(err)
	}
}

func (g *generator) failf(m string, a ...interface{}) {
	panic(fmt.Errorf(m, a...))
}

func (g *generator) fail(err error) {
	panic(err)
}

func (g *generator) genNode(node ast.Node) {
	g.printf("\n")
	switch def := node.(type) {
	case *ast.ObjectDefinition:
		g.genObjectDefinition(def)
		g.printf("\n")
		g.genObjectModel(def)
	case *ast.InputObjectDefinition:
		g.genInputObjectDefinition(def)
		g.printf("\n")
		g.genInputModel(def)
	case *ast.EnumDefinition:
		g.genEnumConstants(def)
		g.printf("\n")
		g.genEnumDefinition(def)
	case *ast.InterfaceDefinition:
		g.genInterfaceDefinition(def)
		g.printf("\n")
		g.genInterfaceModel(def)
	case *ast.UnionDefinition:
		g.genUnionDefinition(def)
		g.printf("\n")
		g.genUnionModel(def)
	default:
		log.Fatalf("Unhandled node type %T", node)
	}
}

func (g *generator) genInterfaceDefinition(def *ast.InterfaceDefinition) {
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	}
	goName := goInterfaceDefName(def.Name.Value)
	g.printf("var %s = graphql.NewInterface(graphql.InterfaceConfig{\n", goName)
	g.printf("\tName: %q,\n", def.Name.Value)
	if def.Doc != nil {
		g.printf("\tDescription: %s,\n", renderQuotedComments(def.Doc))
	}
	g.printf("\tFields: graphql.Fields{\n")
	for _, f := range def.Fields {
		g.printf("%s,\n", g.renderFieldDefinition(def.Name.Value, f, "\t\t", false))
	}
	g.printf("\t},\n")
	g.printf("})\n\n")

	// Generate ResolveType if there's any types that implement this interface
	var objDefs []*ast.ObjectDefinition
	for _, node := range g.doc.Definitions {
		if d, ok := node.(*ast.ObjectDefinition); ok {
			for _, impl := range d.Interfaces {
				if impl.Name.Value == def.Name.Value {
					objDefs = append(objDefs, d)
				}
			}
		}
	}
	if len(objDefs) != 0 {
		g.printf("\nfunc init() {\n")
		g.printf("\t// Resolve the type of an interface value. This done here rather than at declaration time to avoid an unresolvable compile time decleration loop.\n")
		g.printf("\t%s.ResolveType = func(p graphql.ResolveTypeParams) *graphql.Object {\n", goName)
		g.printf("\t\tswitch p.Value.(type) {\n")
		for _, def := range objDefs {
			name := exportedName(def.Name.Value)
			g.printf("\t\tcase *%s:\n", name)
			g.printf("\t\t\treturn %s\n", goObjectDefName(name))
		}
		g.printf("\t\t}\n")
		g.printf("\t\treturn nil\n")
		g.printf("\t}\n")
		g.printf("}\n")
	}
}

func (g *generator) genInterfaceModel(def *ast.InterfaceDefinition) {
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	}
	// TODO: do we want anything here to make guarantees of match?
	g.printf("type %s interface {\n", exportedName(def.Name.Value))
	g.printf("\t// Use an unexported method to guarantee the type to the interface\n")
	g.printf("\t%s()\n", interfaceMarker(def.Name.Value))
	g.printf("}\n")
}

func (g *generator) genUnionDefinition(def *ast.UnionDefinition) {
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	}
	g.printf("var %s = graphql.NewUnion(graphql.UnionConfig{\n", goUnionDefName(def.Name.Value))
	g.printf("\tName: %q,\n", def.Name.Value)
	if def.Doc != nil {
		g.printf("\tDescription: %s,\n", renderQuotedComments(def.Doc))
	}
	g.printf("\tTypes: []*graphql.Object{\n")
	for _, f := range def.Types {
		g.printf("\t\t%s,\n", goObjectDefName(f.Name.Value))
	}
	g.printf("\t},\n")
	g.printf("})\n")
}

func (g *generator) genUnionModel(def *ast.UnionDefinition) {
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	}
	// TODO: do we want anything here to make guarantees of match?
	g.printf("type %s interface {\n", exportedName(def.Name.Value))
	g.printf("}\n")
}

func (g *generator) genEnumDefinition(def *ast.EnumDefinition) {
	goName := exportedName(def.Name.Value)
	goDefName := goEnumDefName(def.Name.Value)

	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	}
	g.printf("var %s = graphql.NewEnum(graphql.EnumConfig{\n", goDefName)
	g.printf("\tName: %q,\n", def.Name.Value)
	if def.Doc != nil {
		g.printf("\tDescription: %s,\n", renderQuotedComments(def.Doc))
	}
	g.printf("\tValues: graphql.EnumValueConfigMap{\n")
	for _, v := range def.Values {
		goConstName := goName + exportedCamelCase(v.Name.Value)
		g.printf("\t\tstring(%s): &graphql.EnumValueConfig{\n", goConstName)
		g.printf("\t\t\tValue: %s,\n", goConstName)
		var comments []*ast.Comment
		if v.Doc != nil {
			comments = append(comments, v.Doc.List...)
		}
		if v.Comment != nil {
			comments = append(comments, v.Comment.List...)
		}
		if len(comments) != 0 {
			g.printf("\t\t\tDescription: %s,\n", renderQuotedComments(&ast.CommentGroup{List: comments}))
		}
		g.printf("\t\t},\n")
	}
	g.printf("\t},\n")
	g.printf("})\n")
}

func (g *generator) genEnumConstants(def *ast.EnumDefinition) {
	goName := exportedName(def.Name.Value)
	goDefName := goEnumDefName(def.Name.Value)

	g.printf("type %s string\n", goName)

	g.printf("\n// Possible values for the %s enum.\n", goDefName)
	g.printf("const (\n")
	for _, v := range def.Values {
		if v.Doc != nil {
			c, _ := renderLineComments(v.Doc, "\t")
			g.printf("%s\n", c)
		}
		var comm string
		if v.Comment != nil {
			comm, _ = renderLineComments(v.Comment, " ")
		}
		g.printf("\t%s%s %s = %q%s\n", goName, exportedCamelCase(v.Name.Value), goName, v.Name.Value, comm)
	}
	g.printf(")\n")
}

func (g *generator) genObjectDefinition(def *ast.ObjectDefinition) {
	goName := goObjectDefName(def.Name.Value)
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	} else if strings.HasSuffix(def.Name.Value, "Payload") {
		g.printf("// %s is the return type for the %s mutation.\n", goName, unexportedName(def.Name.Value[:len(def.Name.Value)-7]))
	}
	cycleTypes := g.cycleBreaks[def.Name.Value]
	g.printf("var %s = graphql.NewObject(graphql.ObjectConfig{\n", goName)
	g.printf("\tName: %q,\n", def.Name.Value)
	if def.Doc != nil {
		g.printf("\tDescription: %s,\n", renderQuotedComments(def.Doc))
	}
	if len(def.Interfaces) != 0 {
		g.printf("\tInterfaces: []*graphql.Interface{\n")
		for _, inf := range def.Interfaces {
			g.printf("\t\t%s,\n", goInterfaceDefName(inf.Name.Value))
		}
		g.printf("\t},\n")
	}
	g.printf("\tFields: graphql.Fields{\n")
	var stubFields []*ast.FieldDefinition
	for _, f := range def.Fields {
		if _, ok := cycleTypes[g.baseTypeName(f.Type)]; ok {
			// Use a placeholder and set the actual type in an init function to break the cycle
			g.printf("\t\t// Placeholder to break cycle. Actual type defined during init.\n")
			g.printf("%s,\n", g.renderFieldDefinition("",
				&ast.FieldDefinition{
					Name: f.Name,
					Type: &ast.Named{Name: &ast.Name{Value: "String"}},
				}, "\t\t", false))
			stubFields = append(stubFields, f)
		} else {
			g.printf("%s,\n", g.renderFieldDefinition(def.Name.Value, f, "\t\t", false))
		}
	}
	g.printf("\t},\n")
	g.printf("\tIsTypeOf: func(p graphql.IsTypeOfParams) bool {\n")
	g.printf("\t\t_, ok := p.Value.(*%s)\n", exportedName(def.Name.Value))
	g.printf("\t\treturn ok\n")
	g.printf("\t},\n")
	g.printf("})\n")

	if len(stubFields) != 0 {
		g.printf("func init() {\n")
		g.printf("\t// Create actual types for fields that can't be created during declartion because they're recursive\n")
		for _, f := range stubFields {
			g.printf("\t%s.AddFieldConfig(%q, %s)\n", goName, f.Name.Value, g.renderFieldDefinition(def.Name.Value, f, "\t\t", true))
		}
		g.printf("}\n")
	}
}

func (g *generator) genObjectModel(def *ast.ObjectDefinition) {
	goName := exportedName(def.Name.Value)
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	} else if strings.HasSuffix(def.Name.Value, "Payload") {
		g.printf("// %s is the return type for the %s mutation.\n", goName, unexportedName(def.Name.Value[:len(def.Name.Value)-7]))
	}
	g.printf("type %s struct {\n", goName)
	for _, f := range def.Fields {
		if !g.hasCustomResolver(def.Name.Value, f.Name.Value) {
			opts := []string{f.Name.Value}
			if _, ok := f.Type.(*ast.NonNull); !ok {
				opts = append(opts, "omitempty")
			}
			g.printf("\t%s %s `json:%q`\n", exportedName(f.Name.Value), g.goType(f.Type, def.Name.Value+"."+f.Name.Value), strings.Join(opts, ","))
		}
	}
	// Turn the ExtraFields map into a slice to make the ordering consistent
	extraFields := make([][2]string, 0, len(g.cfg.ExtraFields[def.Name.Value]))
	for name, goType := range g.cfg.ExtraFields[def.Name.Value] {
		extraFields = append(extraFields, [2]string{name, goType})
	}
	sort.Slice(extraFields, func(i, j int) bool { return extraFields[i][0] < extraFields[j][0] })
	for _, nameAndGoType := range extraFields {
		g.printf("\t%s %s `json:\"-\"`\n", nameAndGoType[0], nameAndGoType[1])
	}
	g.printf("}\n")

	if len(def.Interfaces) != 0 {
		g.printf("\n// Use unexported methods to guarantee struct to interface.\n")
		for _, intf := range def.Interfaces {
			g.printf("func (*%s) %s() {}\n", goName, interfaceMarker(intf.Name.Value))
		}
	}

	// Generate any argument structs
	for _, f := range def.Fields {
		if len(f.Arguments) != 0 {
			g.printf("\ntype %s%sArgs struct {\n", goName, exportedName(f.Name.Value))
			for _, a := range f.Arguments {
				opts := []string{a.Name.Value}
				if _, ok := a.Type.(*ast.NonNull); ok {
					opts = append(opts, "nonempty")
				}
				g.printf("\t%s %s `gql:%q`\n", exportedName(a.Name.Value), g.goType(a.Type, def.Name.Value+"."+f.Name.Value), strings.Join(opts, ","))
			}
			g.printf("}\n")
		}
	}
}

func (g *generator) hasCustomResolver(typeName, fieldName string) bool {
	for _, f := range g.cfg.Resolvers[typeName] {
		if f == fieldName {
			return true
		}
	}
	return false
}

func isTopLevelObject(o string) bool {
	switch o {
	case "Mutation", "Query":
		return true
	}
	return false
}

func (g *generator) renderFieldDefinition(objName string, def *ast.FieldDefinition, indent string, noName bool) string {
	comments := def.Doc
	comment, _ := renderLineComments(def.Comment, indent)
	deprecated := strings.Contains(strings.ToLower(comment), "deprecated")
	customResolve := g.hasCustomResolver(objName, def.Name.Value)
	if comments == nil && len(def.Arguments) == 0 && !deprecated && !customResolve {
		if comment != "" {
			comment += "\n"
		}
		if noName {
			return fmt.Sprintf("&graphql.Field{Type: %s}", g.renderType(def.Type))
		}
		return fmt.Sprintf("%s%s%q: &graphql.Field{Type: %s}", comment, indent, def.Name.Value, g.renderType(def.Type))
	}
	var lines []string
	if !noName && comment != "" && !deprecated {
		lines = append(lines, comment)
	}
	if noName {
		lines = append(lines, "&graphql.Field{")
	} else {
		lines = append(lines, fmt.Sprintf("%s%q: &graphql.Field{", indent, def.Name.Value))
	}
	lines = append(lines, fmt.Sprintf("%s\tType: %s,", indent, g.renderType(def.Type)))

	if len(def.Arguments) != 0 {
		lines = append(lines, indent+"\tArgs: graphql.FieldConfigArgument{")
		for _, a := range def.Arguments {
			lines = append(lines, g.renderArgumentConfig(a, indent+"\t\t")+",")
		}
		lines = append(lines, indent+"\t},")
	}
	if def.Doc != nil {
		lines = append(lines, fmt.Sprintf("%s\tDescription: %s,", indent, renderQuotedComments(def.Doc)))
	}
	if deprecated {
		lines = append(lines, fmt.Sprintf("%s\tDeprecationReason: %s,", indent, renderDeprecationReason(def.Comment)))
	}
	if customResolve {
		goFieldName := exportedName(def.Name.Value)
		goObjName := exportedName(objName)
		assertionType := fmt.Sprintf("*%s", goObjName)
		if isTopLevelObject(goObjName) {
			assertionType = "map[string]interface{}"
		}
		lines = append(lines,
			fmt.Sprintf("%s\tResolve: func(p graphql.ResolveParams) (interface{}, error) {", indent),
			fmt.Sprintf("%s\t\tr := p.Info.RootValue.(map[string]interface{})[%s].(%s)", indent, goObjName+"ResolversKey", goObjName+"Resolvers"))
		if len(def.Arguments) == 0 {
			lines = append(lines, fmt.Sprintf("%s\t\treturn r.%s(p.Context, p.Source.(%s), p)", indent, goFieldName, assertionType))
		} else {
			lines = append(lines,
				fmt.Sprintf("%s\t\tvar args %s%sArgs", indent, goObjName, goFieldName),
				fmt.Sprintf("%s\t\tif err := gqldecode.Decode(p.Args, &args); err != nil {", indent),
				fmt.Sprintf("%s\t\t\tswitch err := err.(type) {", indent),
				fmt.Sprintf("%s\t\t\tcase gqldecode.ErrValidationFailed:", indent),
				fmt.Sprintf("%s\t\t\t\t	return nil, gqlerrors.FormatError(fmt.Errorf(\"%%s is invalid: %%s\", err.Field, err.Reason))", indent),
				fmt.Sprintf("%s\t\t\t}", indent),
				fmt.Sprintf("%s\t\t\treturn nil, err", indent),
				fmt.Sprintf("%s\t\t}", indent),
				fmt.Sprintf("%s\t\treturn r.%s(p.Context, p.Source.(%s), &args, p)", indent, goFieldName, assertionType))
		}
		lines = append(lines, fmt.Sprintf("%s\t},", indent))
	}
	lines = append(lines, indent+"}")
	return strings.Join(lines, "\n")
}

func (g *generator) genInputObjectDefinition(def *ast.InputObjectDefinition) {
	goDefName := goInputObjectDefName(def.Name.Value)
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	} else if strings.HasSuffix(def.Name.Value, "Input") {
		g.printf("// %s is the input type for the %s mutation.\n", goDefName, unexportedName(def.Name.Value[:len(def.Name.Value)-5]))
	}
	g.printf("var %s = graphql.NewInputObject(graphql.InputObjectConfig{\n", goDefName)
	g.printf("\tName: %s,\n", strconv.Quote(def.Name.Value))
	if def.Doc != nil {
		g.printf("\tDescription: %s,\n", renderQuotedComments(def.Doc))
	}
	g.printf("\tFields: graphql.InputObjectConfigFieldMap{\n")
	for _, f := range def.Fields {
		g.printf("%s,\n", g.renderInputValueDefinition(f, "\t\t"))
	}
	g.printf("\t},\n")
	g.printf("})\n")
}

func (g *generator) genInputModel(def *ast.InputObjectDefinition) {
	if def.Doc != nil {
		c, _ := renderLineComments(def.Doc, "")
		g.printf("%s\n", c)
	} else if strings.HasSuffix(def.Name.Value, "Input") {
		g.printf("// %s is the input type for the %s mutation.\n", def.Name.Value, unexportedName(def.Name.Value[:len(def.Name.Value)-5]))
	}
	g.printf("type %s struct {\n", exportedName(def.Name.Value))
	for _, f := range def.Fields {
		iType := g.goType(f.Type, def.Name.Value+"."+f.Name.Value)
		if *flagNullableInputs {
			iType = g.goInputType(f.Type, def.Name.Value+"."+f.Name.Value, true)
		}
		g.printf("\t%s %s `gql:%q`\n", exportedName(f.Name.Value), iType, f.Name.Value)
	}
	g.printf("}\n")
}

func (g *generator) renderInputValueDefinition(def *ast.InputValueDefinition, indent string) string {
	// TODO: default value
	comment, _ := renderLineComments(def.Comment, indent)
	if def.Doc == nil {
		if comment != "" {
			comment += "\n"
		}
		return fmt.Sprintf("%s%s%q: &graphql.InputObjectFieldConfig{Type: %s}", comment, indent, def.Name.Value, g.renderType(def.Type))
	}
	var lines []string
	if comment != "" {
		lines = append(lines, comment)
	}
	lines = append(lines,
		fmt.Sprintf("%s%q: &graphql.InputObjectFieldConfig{", indent, def.Name.Value),
		fmt.Sprintf("%s\tType: %s,", indent, g.renderType(def.Type)),
		fmt.Sprintf("%s\tDescription: %s,", indent, renderQuotedComments(def.Doc)),
		indent+"}")
	return strings.Join(lines, "\n")
}

func (g *generator) renderArgumentConfig(def *ast.InputValueDefinition, indent string) string {
	// TODO: default value
	comment, _ := renderLineComments(def.Comment, indent)
	if def.Doc == nil {
		if comment != "" {
			comment += "\n"
		}
		return fmt.Sprintf("%s%s%q: &graphql.ArgumentConfig{Type: %s}", comment, indent, def.Name.Value, g.renderType(def.Type))
	}
	var lines []string
	if comment != "" {
		lines = append(lines, comment)
	}
	lines = append(lines,
		fmt.Sprintf("%s%q: &graphql.ArgumentConfig{", indent, def.Name.Value),
		fmt.Sprintf("%s\tType: %s,", indent, g.renderType(def.Type)),
		fmt.Sprintf("%s\tDescription: %s,", indent, renderQuotedComments(def.Doc)),
		indent+"}")
	return strings.Join(lines, "\n")
}

func (g *generator) renderType(t ast.Type) string {
	switch t := t.(type) {
	case *ast.NonNull:
		return "graphql.NewNonNull(" + g.renderType(t.Type) + ")"
	case *ast.List:
		return "graphql.NewList(" + g.renderType(t.Type) + ")"
	case *ast.Named:
		switch t.Name.Value {
		case "ID", "String", "Boolean", "Float", "Int":
			return "graphql." + t.Name.Value
		}
		// Make sure the type exists
		node, ok := g.types[t.Name.Value]
		if !ok {
			g.failf("Undefined type %q", t.Name.Value)
		}
		switch node.(type) {
		case *ast.ObjectDefinition:
			return goObjectDefName(t.Name.Value)
		case *ast.InterfaceDefinition:
			return goInterfaceDefName(t.Name.Value)
		case *ast.UnionDefinition:
			return goUnionDefName(t.Name.Value)
		case *ast.EnumDefinition:
			return goEnumDefName(t.Name.Value)
		case *ast.InputObjectDefinition:
			return goInputObjectDefName(t.Name.Value)
		}
		g.failf("Unknown node type %T", node)
	}
	log.Fatalf("Unhandled type %T", t)
	return ""
}

func renderDefaultReturnValue(t ast.Type) string {
	switch t := t.(type) {
	case *ast.NonNull:
		return renderDefaultReturnValue(t.Type)
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

func (g *generator) goInputType(t ast.Type, fieldName string, nullable bool) string {
	var p string
	if nullable {
		p = "*"
	}
	if t := g.cfg.CustomFieldTypes[fieldName]; t != "" {
		return t
	}
	switch t := t.(type) {
	case *ast.NonNull:
		return g.goInputType(t.Type, fieldName, false)
	case *ast.List:
		return "[]" + g.goInputType(t.Type, fieldName, true)
	case *ast.Named:
		switch t.Name.Value {
		case "ID":
			return p + "string"
		case "String":
			return p + "string"
		case "Boolean":
			return p + "bool"
		case "Float":
			return p + "float64"
		case "Int":
			return p + "int"
		}
		node, ok := g.types[t.Name.Value]
		if !ok {
			g.failf("Undefined type %q", t.Name.Value)
		}
		if _, ok := node.(*ast.EnumDefinition); ok {
			return p + exportedName(t.Name.Value)
		}
		if _, ok := node.(*ast.InterfaceDefinition); ok {
			return exportedName(t.Name.Value)
		}
		return "*" + exportedName(t.Name.Value)
	}
	log.Fatalf("Unhandled type %T", t)
	return ""
}

func (g *generator) goType(t ast.Type, fieldName string) string {
	if t := g.cfg.CustomFieldTypes[fieldName]; t != "" {
		return t
	}
	switch t := t.(type) {
	case *ast.NonNull:
		return g.goType(t.Type, fieldName)
	case *ast.List:
		return "[]" + g.goType(t.Type, fieldName)
	case *ast.Named:
		switch t.Name.Value {
		case "ID":
			return "string"
		case "String":
			return "string"
		case "Boolean":
			return "bool"
		case "Float":
			return "float64"
		case "Int":
			if strings.HasSuffix(strings.ToLower(fieldName), "timestamp") {
				return "int64"
			}
			return "int"
		}
		node, ok := g.types[t.Name.Value]
		if !ok {
			g.failf("Undefined type %q", t.Name.Value)
		}
		if _, ok := node.(*ast.EnumDefinition); ok {
			return exportedName(t.Name.Value)
		}
		if _, ok := node.(*ast.InterfaceDefinition); ok {
			return exportedName(t.Name.Value)
		}
		if _, ok := node.(*ast.UnionDefinition); ok {
			return exportedName(t.Name.Value)
		}
		return "*" + exportedName(t.Name.Value)
	}
	log.Fatalf("Unhandled type %T", t)
	return ""
}

func renderLineComments(cg *ast.CommentGroup, indent string) (string, directives) {
	if cg == nil {
		return "", nil
	}
	lines := make([]string, len(cg.List))
	for i, c := range cg.List {
		lines[i] = indent + "// " + strings.TrimLeft(c.Text, "# ")
	}
	return strings.Join(lines, "\n"), nil
}

func renderQuotedComments(cg *ast.CommentGroup) string {
	lines := make([]string, len(cg.List))
	for i, c := range cg.List {
		lines[i] = strings.TrimLeft(c.Text, "# ")
	}
	text := strings.Join(lines, "\n")
	if strings.ContainsRune(text, '\n') {
		return "`" + strings.Replace(text, "`", "'", -1) + "`"
	}
	return strconv.Quote(text)
}

func renderDeprecationReason(cg *ast.CommentGroup) string {
	lines := make([]string, len(cg.List))
	for i, c := range cg.List {
		lines[i] = strings.TrimLeft(c.Text, "# ")
	}
	text := strings.Join(lines, "\n")
	if strings.ContainsRune(text, '\n') {
		return "`" + strings.Replace(text, "`", "'", -1) + "`"
	}
	upperText := strings.ToUpper(text)
	if strings.HasPrefix(upperText, "DEPRECATED.") || strings.HasPrefix(upperText, "DEPRECATED:") {
		text = strings.TrimSpace(text[11:])
	}
	return strconv.Quote(text)
}

func interfaceMarker(typeName string) string {
	return unexportedName(typeName) + "Marker"
}

func goObjectDefName(name string) string {
	return exportedName(name) + "Def"
}

func goInterfaceDefName(name string) string {
	return exportedName(name) + "Def"
}

func goEnumDefName(name string) string {
	return exportedName(name) + "Def"
}

func goUnionDefName(name string) string {
	return exportedName(name) + "Def"
}

func goInputObjectDefName(name string) string {
	return exportedName(name) + "Def"
}

// unexportedName lowercases the first word of a camelcased name
func unexportedName(s string) string {
	if s == "" {
		return s
	}
	for i, r := range s {
		if unicode.IsLower(r) {
			if i == 0 {
				return s
			}
			if _, ok := initialisms[s[:i-1]]; ok {
				return strings.ToLower(s[:i-1]) + s[i-1:]
			}
			return strings.ToLower(s[:1]) + s[1:]
		}
	}
	return strings.ToLower(s)
}

// exportedName guarantees the first character of the name is uppercases
func exportedName(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	if !unicode.IsUpper(r) {
		s = string(unicode.ToUpper(r)) + s[n:]
	}
	return camelCaseInitialisms(s)
}

// exportedCamelCase returns the string converted to camel case (e.g. some_name to SomeName)
func exportedCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = upperInitialisms(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
		}
	}
	return strings.Join(parts, "")
}

var reCamelCase = regexp.MustCompile(`[A-Z][^A-Z]+`)

// camelCaseInitialisms takes a camel case string and convert any initialisms to uppercase (e.g. ObjectId -> ObjectID)
func camelCaseInitialisms(s string) string {
	return reCamelCase.ReplaceAllStringFunc(s, func(s string) string {
		return upperInitialisms(s)
	})
}

// upperInitialisms takes a word and convert any initialisms to uppercase (e.g. Url -> URL)
func upperInitialisms(s string) string {
	x := strings.ToUpper(s)
	if y, ok := initialisms[x]; ok {
		return y
	}
	return s
}
