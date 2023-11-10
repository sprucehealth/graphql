package graphql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
)

type ExecuteParams struct {
	Schema            Schema
	Root              interface{}
	AST               *ast.Document
	OperationName     string
	Args              map[string]interface{}
	DeprecatedFieldFn func(ctx context.Context, parent *Object, fieldDef *FieldDefinition) error
	// TODO: Abstract this to possibly handle more types
	FieldDefinitionDirectiveHandler func(context.Context, *ast.Directive, *FieldDefinition) error
	DisallowIntrospection           bool
	// TimeoutWait is the amount of time to allow for resolvers to handle
	// a context deadline error before the executor does.
	TimeoutWait time.Duration
}

func Execute(ctx context.Context, p ExecuteParams) *Result {
	resultChannel := make(chan *Result, 1)

	go func(out chan<- *Result, done <-chan struct{}) {
		result := &Result{}

		exeContext, err := buildExecutionContext(BuildExecutionCtxParams{
			Schema:                          p.Schema,
			Root:                            p.Root,
			AST:                             p.AST,
			OperationName:                   p.OperationName,
			Args:                            p.Args,
			Errors:                          nil,
			Result:                          result,
			DeprecatedFieldFn:               p.DeprecatedFieldFn,
			FieldDefinitionDirectiveHandler: p.FieldDefinitionDirectiveHandler,
			DisallowIntrospection:           p.DisallowIntrospection,
		})

		if err != nil {
			result.Errors = append(result.Errors, gqlerrors.FormatError(err))
			out <- result
			return
		}

		defer func() {
			if r := recover(); r != nil {
				err := gqlerrors.FormatPanic(r)
				exeContext.Errors = append(exeContext.Errors, gqlerrors.FormatError(err))
				result.Errors = exeContext.Errors
			}
			out <- result
		}()

		result = executeOperation(ctx, ExecuteOperationParams{
			ExecutionContext: exeContext,
			Root:             p.Root,
			Operation:        exeContext.Operation,
		})
	}(resultChannel, ctx.Done())

	var result *Result
	select {
	case r := <-resultChannel:
		result = r
	case <-ctx.Done():
		err := ctx.Err()
		if errors.Is(err, context.DeadlineExceeded) && p.TimeoutWait != 0 {
			select {
			case r := <-resultChannel:
				result = r
			case <-time.After(p.TimeoutWait):
			}
		}
		if result == nil {
			result = &Result{}
			result.Errors = append(result.Errors, gqlerrors.FormatError(err))
		}
	}
	return result
}

type BuildExecutionCtxParams struct {
	Schema            Schema
	Root              interface{}
	AST               *ast.Document
	OperationName     string
	Args              map[string]interface{}
	Errors            []gqlerrors.FormattedError
	Result            *Result
	DeprecatedFieldFn func(context.Context, *Object, *FieldDefinition) error
	// TODO: Abstract this to possibly handle more types
	FieldDefinitionDirectiveHandler func(context.Context, *ast.Directive, *FieldDefinition) error
	DisallowIntrospection           bool
}

type ExecutionContext struct {
	Schema            Schema
	Fragments         map[string]*ast.FragmentDefinition
	Root              interface{}
	Operation         ast.Definition
	VariableValues    map[string]interface{}
	Errors            []gqlerrors.FormattedError
	DeprecatedFieldFn func(context.Context, *Object, *FieldDefinition) error
	// TODO: Abstract this to possibly handle more types
	FieldDefinitionDirectiveHandler func(context.Context, *ast.Directive, *FieldDefinition) error
	DisallowIntrospection           bool
}

func safeNodeType(n ast.Node) string {
	return strings.TrimPrefix(reflect.TypeOf(n).String(), "*ast.")
}

func buildExecutionContext(p BuildExecutionCtxParams) (*ExecutionContext, error) {
	var operation *ast.OperationDefinition
	fragments := make(map[string]*ast.FragmentDefinition)
	for _, definition := range p.AST.Definitions {
		switch definition := definition.(type) {
		case *ast.OperationDefinition:
			if p.OperationName == "" && operation != nil {
				return nil, errors.New("Must provide operation name if query contains multiple operations.")
			}
			if p.OperationName == "" || definition.GetName() != nil && definition.GetName().Value == p.OperationName {
				operation = definition
			}
		case *ast.FragmentDefinition:
			key := ""
			if definition.GetName() != nil && definition.GetName().Value != "" {
				key = definition.GetName().Value
			}
			fragments[key] = definition
		default:
			return nil, fmt.Errorf("GraphQL cannot execute a request containing a %s", safeNodeType(definition))
		}
	}

	if operation == nil {
		if p.OperationName != "" {
			return nil, fmt.Errorf("Unknown operation named %q.", p.OperationName)
		}
		return nil, errors.New("Must provide an operation.")
	}

	variableValues, err := getVariableValues(p.Schema, operation.GetVariableDefinitions(), p.Args)
	if err != nil {
		return nil, err
	}

	return &ExecutionContext{
		Schema:                          p.Schema,
		Fragments:                       fragments,
		Root:                            p.Root,
		Operation:                       operation,
		VariableValues:                  variableValues,
		Errors:                          p.Errors,
		DeprecatedFieldFn:               p.DeprecatedFieldFn,
		FieldDefinitionDirectiveHandler: p.FieldDefinitionDirectiveHandler,
		DisallowIntrospection:           p.DisallowIntrospection,
	}, nil
}

type ExecuteOperationParams struct {
	ExecutionContext *ExecutionContext
	Root             interface{}
	Operation        ast.Definition
}

func executeOperation(ctx context.Context, p ExecuteOperationParams) *Result {
	operationType, err := getOperationRootType(p.ExecutionContext.Schema, p.Operation)
	if err != nil {
		return &Result{Errors: gqlerrors.FormatErrors(err)}
	}

	fields := collectFields(CollectFieldsParams{
		ExeContext:   p.ExecutionContext,
		RuntimeType:  operationType,
		SelectionSet: p.Operation.GetSelectionSet(),
	})

	executeFieldsParams := ExecuteFieldsParams{
		ExecutionContext: p.ExecutionContext,
		ParentType:       operationType,
		Source:           p.Root,
		Fields:           fields,
	}

	if p.Operation.GetOperation() == ast.OperationTypeMutation {
		return executeFieldsSerially(ctx, executeFieldsParams)
	}
	return executeFields(ctx, executeFieldsParams)
}

// Extracts the root type of the operation from the schema.
func getOperationRootType(schema Schema, operation ast.Definition) (*Object, error) {
	if operation == nil {
		return nil, errors.New("Can only execute queries and mutations")
	}

	switch operation.GetOperation() {
	case ast.OperationTypeQuery:
		return schema.QueryType(), nil
	case ast.OperationTypeMutation:
		mutationType := schema.MutationType()
		if mutationType.PrivateName == "" {
			return nil, gqlerrors.NewError(
				gqlerrors.ErrorTypeBadQuery,
				"Schema is not configured for mutations",
				[]ast.Node{operation},
				"",
				nil,
				[]int{},
				nil,
			)
		}
		return mutationType, nil
	case ast.OperationTypeSubscription:
		subscriptionType := schema.SubscriptionType()
		if subscriptionType.PrivateName == "" {
			return nil, gqlerrors.NewError(
				gqlerrors.ErrorTypeBadQuery,
				"Schema is not configured for subscriptions",
				[]ast.Node{operation},
				"",
				nil,
				[]int{},
				nil,
			)
		}
		return subscriptionType, nil
	}
	return nil, gqlerrors.NewError(
		gqlerrors.ErrorTypeBadQuery,
		"Can only execute queries, mutations and subscription",
		[]ast.Node{operation},
		"",
		nil,
		[]int{},
		nil,
	)
}

type ExecuteFieldsParams struct {
	ExecutionContext *ExecutionContext
	ParentType       *Object
	Source           interface{}
	Fields           map[string][]*ast.Field
}

// Implements the "Evaluating selection sets" section of the spec for "write" mode.
func executeFieldsSerially(ctx context.Context, p ExecuteFieldsParams) *Result {
	if p.Source == nil {
		p.Source = make(map[string]interface{})
	}
	if p.Fields == nil {
		p.Fields = make(map[string][]*ast.Field)
	}

	finalResults := make(map[string]interface{})
	for responseName, fieldASTs := range p.Fields {
		resolved, state := resolveField(ctx, p.ExecutionContext, p.ParentType, p.Source, fieldASTs)
		if state.hasNoFieldDefs {
			continue
		}
		finalResults[responseName] = resolved
	}

	return &Result{
		Data:   finalResults,
		Errors: p.ExecutionContext.Errors,
	}
}

// Implements the "Evaluating selection sets" section of the spec for "read" mode.
func executeFields(ctx context.Context, p ExecuteFieldsParams) *Result {
	if p.Source == nil {
		p.Source = make(map[string]interface{})
	}
	if p.Fields == nil {
		p.Fields = make(map[string][]*ast.Field)
	}

	finalResults := make(map[string]interface{})
	for responseName, fieldASTs := range p.Fields {
		resolved, state := resolveField(ctx, p.ExecutionContext, p.ParentType, p.Source, fieldASTs)
		if state.hasNoFieldDefs {
			continue
		}
		finalResults[responseName] = resolved
	}

	return &Result{
		Data:   finalResults,
		Errors: p.ExecutionContext.Errors,
	}
}

type CollectFieldsParams struct {
	ExeContext           *ExecutionContext
	RuntimeType          *Object // previously known as OperationType
	SelectionSet         *ast.SelectionSet
	Fields               map[string][]*ast.Field
	VisitedFragmentNames map[string]struct{}
}

// Given a selectionSet, adds all of the fields in that selection to
// the passed in map of fields, and returns it at the end.
// CollectFields requires the "runtime type" of an object. For a field which
// returns and Interface or Union type, the "runtime type" will be the actual
// Object type returned by that field.
func collectFields(p CollectFieldsParams) map[string][]*ast.Field {
	fields := p.Fields
	if fields == nil {
		fields = make(map[string][]*ast.Field)
	}
	if p.VisitedFragmentNames == nil {
		p.VisitedFragmentNames = make(map[string]struct{})
	}
	if p.SelectionSet == nil {
		return fields
	}
	for _, iSelection := range p.SelectionSet.Selections {
		switch selection := iSelection.(type) {
		case *ast.Field:
			if !shouldIncludeNode(p.ExeContext, selection.Directives) {
				continue
			}
			name := getFieldEntryKey(selection)
			fields[name] = append(fields[name], selection)
		case *ast.InlineFragment:
			if !shouldIncludeNode(p.ExeContext, selection.Directives) ||
				!doesFragmentConditionMatch(p.ExeContext, selection, p.RuntimeType) {
				continue
			}
			innerParams := CollectFieldsParams{
				ExeContext:           p.ExeContext,
				RuntimeType:          p.RuntimeType,
				SelectionSet:         selection.SelectionSet,
				Fields:               fields,
				VisitedFragmentNames: p.VisitedFragmentNames,
			}
			collectFields(innerParams)
		case *ast.FragmentSpread:
			fragName := ""
			if selection.Name != nil {
				fragName = selection.Name.Value
			}
			if _, ok := p.VisitedFragmentNames[fragName]; ok ||
				!shouldIncludeNode(p.ExeContext, selection.Directives) {
				continue
			}
			p.VisitedFragmentNames[fragName] = struct{}{}
			fragment, hasFragment := p.ExeContext.Fragments[fragName]
			if !hasFragment {
				continue
			}

			if !doesFragmentConditionMatch(p.ExeContext, fragment, p.RuntimeType) {
				continue
			}
			innerParams := CollectFieldsParams{
				ExeContext:           p.ExeContext,
				RuntimeType:          p.RuntimeType,
				SelectionSet:         fragment.GetSelectionSet(),
				Fields:               fields,
				VisitedFragmentNames: p.VisitedFragmentNames,
			}
			collectFields(innerParams)
		}
	}
	return fields
}

// Determines if a field should be included based on the @include and @skip
// directives, where @skip has higher precedence than @include.
func shouldIncludeNode(eCtx *ExecutionContext, directives []*ast.Directive) bool {
	defaultReturnValue := true

	var skipAST *ast.Directive
	var includeAST *ast.Directive
	for _, directive := range directives {
		if directive == nil || directive.Name == nil {
			continue
		}
		if directive.Name.Value == SkipDirective.Name {
			skipAST = directive
			break
		}
	}
	if skipAST != nil {
		argValues := getArgumentValues(
			SkipDirective.Args,
			skipAST.Arguments,
			eCtx.VariableValues,
		)
		if skipIf, ok := argValues["if"].(bool); ok {
			if skipIf {
				return false
			}
		}
	}
	for _, directive := range directives {
		if directive == nil || directive.Name == nil {
			continue
		}
		if directive.Name.Value == IncludeDirective.Name {
			includeAST = directive
			break
		}
	}
	if includeAST != nil {
		argValues := getArgumentValues(
			IncludeDirective.Args,
			includeAST.Arguments,
			eCtx.VariableValues,
		)
		if includeIf, ok := argValues["if"].(bool); ok {
			if !includeIf {
				return false
			}
		}
	}
	return defaultReturnValue
}

// Determines if a fragment is applicable to the given type.
func doesFragmentConditionMatch(eCtx *ExecutionContext, fragment ast.Node, ttype *Object) bool {
	switch fragment := fragment.(type) {
	case *ast.FragmentDefinition:
		if fragment.TypeCondition == nil {
			return true
		}
		conditionalType, err := typeFromAST(eCtx.Schema, fragment.TypeCondition)
		if err != nil {
			return false
		}
		if conditionalType == ttype {
			return true
		}
		if conditionalType.Name() == ttype.Name() {
			return true
		}
		if conditionalType, ok := conditionalType.(*Interface); ok {
			return eCtx.Schema.IsPossibleType(conditionalType, ttype)
		}
		if conditionalType, ok := conditionalType.(*Union); ok {
			return eCtx.Schema.IsPossibleType(conditionalType, ttype)
		}
	case *ast.InlineFragment:
		if fragment.TypeCondition == nil {
			return true
		}
		conditionalType, err := typeFromAST(eCtx.Schema, fragment.TypeCondition)
		if err != nil {
			return false
		}
		if conditionalType == ttype {
			return true
		}
		if conditionalType.Name() == ttype.Name() {
			return true
		}
		if conditionalType, ok := conditionalType.(*Interface); ok {
			return eCtx.Schema.IsPossibleType(conditionalType, ttype)
		}
		if conditionalType, ok := conditionalType.(*Union); ok {
			return eCtx.Schema.IsPossibleType(conditionalType, ttype)
		}
	}

	return false
}

// Implements the logic to compute the key of a given field’s entry
func getFieldEntryKey(node *ast.Field) string {
	if node.Alias != nil && node.Alias.Value != "" {
		return node.Alias.Value
	}
	if node.Name != nil && node.Name.Value != "" {
		return node.Name.Value
	}
	return ""
}

// Internal resolveField state
type resolveFieldResultState struct {
	hasNoFieldDefs bool
}

// Resolves the field on the given source object. In particular, this
// figures out the value that the field returns by calling its resolve function,
// then calls completeValue to complete promises, serialize scalars, or execute
// the sub-selection-set for objects.
func resolveField(ctx context.Context, eCtx *ExecutionContext, parentType *Object, source interface{}, fieldASTs []*ast.Field) (result interface{}, resultState resolveFieldResultState) {
	if err := ctx.Err(); err != nil {
		// Jump straight to the top-level recover to void anymore work.
		panic(gqlerrors.FormatError(err))
	}

	// catch panic from resolveFn
	var returnType Output
	defer func() (interface{}, resolveFieldResultState) {
		if r := recover(); r != nil {
			var err error
			if s, ok := r.(string); ok {
				err = NewLocatedError(s, FieldASTsToNodeASTs(fieldASTs))
			} else {
				err = gqlerrors.FormatPanic(r)
			}
			// send panic upstream
			if _, ok := returnType.(*NonNull); ok {
				panic(gqlerrors.FormatError(err))
			}
			eCtx.Errors = append(eCtx.Errors, gqlerrors.FormatError(err))
			return result, resultState
		}
		return result, resultState
	}()

	fieldAST := fieldASTs[0]
	fieldName := ""
	if fieldAST.Name != nil {
		fieldName = fieldAST.Name.Value
	}

	fieldDef := getFieldDef(eCtx.Schema, parentType, fieldName, eCtx.DisallowIntrospection)
	if fieldDef == nil {
		resultState.hasNoFieldDefs = true
		return nil, resultState
	}

	if fieldDef.DeprecationReason != "" && eCtx.DeprecatedFieldFn != nil {
		if err := eCtx.DeprecatedFieldFn(ctx, parentType, fieldDef); err != nil {
			panic(gqlerrors.FormatError(err))
		}
	}

	if len(fieldDef.Directives) != 0 && eCtx.FieldDefinitionDirectiveHandler != nil {
		for _, d := range fieldDef.Directives {
			if err := eCtx.FieldDefinitionDirectiveHandler(ctx, d, fieldDef); err != nil {
				panic(gqlerrors.FormatError(err))
			}
		}
	}

	returnType = fieldDef.Type
	resolveFn := fieldDef.Resolve
	if resolveFn == nil {
		resolveFn = defaultResolveFn
	}

	// Build a map of arguments from the field.arguments AST, using the
	// variables scope to fulfill any variable references.
	// TODO: find a way to memoize, in case this field is within a List type.
	args := getArgumentValues(fieldDef.Args, fieldAST.Arguments, eCtx.VariableValues)

	info := ResolveInfo{
		FieldName:      fieldName,
		FieldASTs:      fieldASTs,
		ReturnType:     returnType,
		ParentType:     parentType,
		Schema:         eCtx.Schema,
		Fragments:      eCtx.Fragments,
		RootValue:      eCtx.Root,
		Operation:      eCtx.Operation,
		VariableValues: eCtx.VariableValues,
	}

	var resolveFnError error

	result, resolveFnError = resolveFn(ctx, ResolveParams{
		Source: source,
		Args:   args,
		Info:   info,
	})

	if resolveFnError != nil {
		panic(gqlerrors.FormatError(resolveFnError))
	}

	completed := completeValueCatchingError(ctx, eCtx, returnType, fieldASTs, info, result)
	return completed, resultState
}

func completeValueCatchingError(ctx context.Context, eCtx *ExecutionContext, returnType Type, fieldASTs []*ast.Field, info ResolveInfo, result interface{}) (completed interface{}) {
	// catch panic
	defer func() interface{} {
		if r := recover(); r != nil {
			//send panic upstream
			if _, ok := returnType.(*NonNull); ok {
				panic(r)
			}
			if err, ok := r.(gqlerrors.FormattedError); ok {
				eCtx.Errors = append(eCtx.Errors, err)
			}
			return completed
		}
		return completed
	}()

	if returnType, ok := returnType.(*NonNull); ok {
		completed := completeValue(ctx, eCtx, returnType, fieldASTs, info, result)
		return completed
	}
	completed = completeValue(ctx, eCtx, returnType, fieldASTs, info, result)
	return completed
}

func completeValue(ctx context.Context, eCtx *ExecutionContext, returnType Type, fieldASTs []*ast.Field, info ResolveInfo, result interface{}) interface{} {
	if err := ctx.Err(); err != nil {
		panic(gqlerrors.FormatError(err))
	}

	resultVal := reflect.ValueOf(result)
	if resultVal.IsValid() && resultVal.Type().Kind() == reflect.Func {
		if propertyFn, ok := result.(func() interface{}); ok {
			return propertyFn()
		}
		panic(gqlerrors.NewFormattedError("Error resolving func. Expected `func() interface{}` signature"))
	}

	// If field type is NonNull, complete for inner type, and throw field error
	// if result is null.
	if returnType, ok := returnType.(*NonNull); ok {
		completed := completeValue(ctx, eCtx, returnType.OfType, fieldASTs, info, result)
		if completed == nil {
			err := NewLocatedError(
				fmt.Sprintf("Cannot return null for non-nullable field %v.%v.", info.ParentType, info.FieldName),
				FieldASTsToNodeASTs(fieldASTs),
			)
			panic(gqlerrors.FormatError(err))
		}
		return completed
	}

	// If result value is null-ish (null, undefined, or NaN) then return null.
	if isNullish(result) {
		return nil
	}

	// If field type is List, complete each item in the list with the inner type
	if returnType, ok := returnType.(*List); ok {
		return completeListValue(ctx, eCtx, returnType, fieldASTs, info, result)
	}

	// If field type is a leaf type, Scalar or Enum, serialize to a valid value,
	// returning null if serialization is not possible.
	if returnType, ok := returnType.(*Scalar); ok {
		return completeLeafValue(returnType, result)
	}
	if returnType, ok := returnType.(*Enum); ok {
		return completeLeafValue(returnType, result)
	}

	// If field type is an abstract type, Interface or Union, determine the
	// runtime Object type and complete for that type.
	if returnType, ok := returnType.(*Union); ok {
		return completeAbstractValue(ctx, eCtx, returnType, fieldASTs, info, result)
	}
	if returnType, ok := returnType.(*Interface); ok {
		return completeAbstractValue(ctx, eCtx, returnType, fieldASTs, info, result)
	}

	// If field type is Object, execute and complete all sub-selections.
	if returnType, ok := returnType.(*Object); ok {
		return completeObjectValue(ctx, eCtx, returnType, fieldASTs, info, result)
	}

	// Not reachable. All possible output types have been considered.
	panic(gqlerrors.NewFormattedError(fmt.Sprintf(`Cannot complete value of unexpected type "%v."`, returnType)))
}

// completeAbstractValue completes value of an Abstract type (Union / Interface) by determining the runtime type
// of that value, then completing based on that type.
func completeAbstractValue(ctx context.Context, eCtx *ExecutionContext, returnType Abstract, fieldASTs []*ast.Field, info ResolveInfo, result interface{}) interface{} {
	var runtimeType *Object

	resolveTypeParams := ResolveTypeParams{
		Value: result,
		Info:  info,
	}
	if unionReturnType, ok := returnType.(*Union); ok && unionReturnType.ResolveType != nil {
		runtimeType = unionReturnType.ResolveType(ctx, resolveTypeParams)
	} else if interfaceReturnType, ok := returnType.(*Interface); ok && interfaceReturnType.ResolveType != nil {
		runtimeType = interfaceReturnType.ResolveType(ctx, resolveTypeParams)
	} else {
		runtimeType = defaultResolveTypeFn(resolveTypeParams, returnType)
	}

	if runtimeType == nil {
		panic(gqlerrors.NewFormattedError(
			fmt.Sprintf(`Abstract type %v must resolve to an Object type at runtime `+
				`for field %v.%v with value "%v", received "%v".`,
				returnType, info.ParentType, info.FieldName, result, runtimeType)))
	}

	if !eCtx.Schema.IsPossibleType(returnType, runtimeType) {
		panic(gqlerrors.NewFormattedError(
			fmt.Sprintf(`Runtime Object type "%v" is not a possible type `+
				`for "%v".`, runtimeType, returnType),
		))
	}

	return completeObjectValue(ctx, eCtx, runtimeType, fieldASTs, info, result)
}

// completeObjectValue complete an Object value by executing all sub-selections.
func completeObjectValue(ctx context.Context, eCtx *ExecutionContext, returnType *Object, fieldASTs []*ast.Field, info ResolveInfo, result interface{}) interface{} {
	// If there is an isTypeOf predicate function, call it with the
	// current result. If isTypeOf returns false, then raise an error rather
	// than continuing execution.
	if returnType.IsTypeOf != nil {
		p := IsTypeOfParams{
			Value: result,
			Info:  info,
		}
		if !returnType.IsTypeOf(p) {
			panic(gqlerrors.NewFormattedError(
				fmt.Sprintf(`Expected value of type "%v" but got: %T.`, returnType, result),
			))
		}
	}

	// Collect sub-fields to execute to complete this value.
	subFieldASTs := make(map[string][]*ast.Field)
	visitedFragmentNames := make(map[string]struct{})
	for _, fieldAST := range fieldASTs {
		if fieldAST == nil {
			continue
		}
		selectionSet := fieldAST.SelectionSet
		if selectionSet != nil {
			innerParams := CollectFieldsParams{
				ExeContext:           eCtx,
				RuntimeType:          returnType,
				SelectionSet:         selectionSet,
				Fields:               subFieldASTs,
				VisitedFragmentNames: visitedFragmentNames,
			}
			subFieldASTs = collectFields(innerParams)
		}
	}
	executeFieldsParams := ExecuteFieldsParams{
		ExecutionContext: eCtx,
		ParentType:       returnType,
		Source:           result,
		Fields:           subFieldASTs,
	}
	results := executeFields(ctx, executeFieldsParams)

	return results.Data

}

// completeLeafValue complete a leaf value (Scalar / Enum) by serializing to a valid value, returning nil if serialization is not possible.
func completeLeafValue(returnType Leaf, result interface{}) interface{} {
	serializedResult := returnType.Serialize(result)
	if isNullish(serializedResult) {
		return nil
	}
	return serializedResult
}

// completeListValue complete a list value by completing each item in the list with the inner type
func completeListValue(ctx context.Context, eCtx *ExecutionContext, returnType *List, fieldASTs []*ast.Field, info ResolveInfo, result interface{}) interface{} {
	resultVal := reflect.ValueOf(result)
	parentTypeName := ""
	if info.ParentType != nil {
		parentTypeName = info.ParentType.Name()
	}
	if !resultVal.IsValid() || resultVal.Type().Kind() != reflect.Slice {
		panic(gqlerrors.NewFormattedError(fmt.Sprintf("User Error: expected iterable, but did not find one for field %v.%v.", parentTypeName, info.FieldName)))
	}

	itemType := returnType.OfType
	completedResults := make([]interface{}, 0, resultVal.Len())
	for i := 0; i < resultVal.Len(); i++ {
		val := resultVal.Index(i).Interface()
		completedItem := completeValueCatchingError(ctx, eCtx, itemType, fieldASTs, info, val)
		completedResults = append(completedResults, completedItem)
	}
	return completedResults
}

type structFieldInfo struct {
	index     int
	omitempty bool
}

var (
	structTypeCacheMu sync.RWMutex
	structTypeCache   = make(map[reflect.Type]map[string]structFieldInfo) // struct type -> field name -> field info
)

func fieldInfoForStruct(structType reflect.Type) map[string]structFieldInfo {
	structTypeCacheMu.RLock()
	sm := structTypeCache[structType]
	structTypeCacheMu.RUnlock()
	if sm != nil {
		return sm
	}

	// Cache a mapping of fields for the struct

	structTypeCacheMu.Lock()
	defer structTypeCacheMu.Unlock()

	// Check again in case someone beat us
	sm = structTypeCache[structType]
	if sm != nil {
		return sm
	}

	sm = make(map[string]structFieldInfo)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		tag := field.Tag
		t := tag.Get("graphql")
		if t == "" {
			t = tag.Get("json")
		}
		tOpts := strings.Split(t, ",")
		if len(tOpts) == 0 {
			sm[field.Name] = structFieldInfo{index: i}
		} else {
			omitempty := len(tOpts) > 1 && tOpts[1] == "omitempty"
			sm[field.Name] = structFieldInfo{index: i, omitempty: omitempty}
			sm[tOpts[0]] = structFieldInfo{index: i, omitempty: omitempty}
		}
	}
	structTypeCache[structType] = sm
	return sm
}

// defaultResolveTypeFn If a resolveType function is not given, then a default resolve behavior is
// used which tests each possible type for the abstract type by calling
// isTypeOf for the object being coerced, returning the first type that matches.
func defaultResolveTypeFn(p ResolveTypeParams, abstractType Abstract) *Object {
	possibleTypes := p.Info.Schema.PossibleTypes(abstractType)
	for _, possibleType := range possibleTypes {
		if possibleType.IsTypeOf == nil {
			continue
		}
		if res := possibleType.IsTypeOf(IsTypeOfParams(p)); res {
			return possibleType
		}
	}
	return nil
}

// defaultResolveFn If a resolve function is not given, then a default resolve behavior is used
// which takes the property of the source object of the same name as the field
// and returns it as the result, or if it's a function, returns the result
// of calling that function.
func defaultResolveFn(ctx context.Context, p ResolveParams) (interface{}, error) {
	// try p.Source as a map[string]interface
	if sourceMap, ok := p.Source.(map[string]interface{}); ok {
		property := sourceMap[p.Info.FieldName]
		if fn, ok := property.(func() interface{}); ok {
			return fn(), nil
		}
		return property, nil
	}

	// try to resolve p.Source as a struct first
	sourceVal := reflect.ValueOf(p.Source)
	if sourceVal.IsValid() && sourceVal.Type().Kind() == reflect.Ptr {
		sourceVal = sourceVal.Elem()
	}
	if !sourceVal.IsValid() {
		return nil, nil
	}
	sourceType := sourceVal.Type()
	if sourceType.Kind() == reflect.Struct {
		sm := fieldInfoForStruct(sourceType)
		if field, ok := sm[p.Info.FieldName]; ok {
			valueField := sourceVal.Field(field.index)
			if field.omitempty && isEmptyValue(valueField) {
				return nil, nil
			}
			return valueField.Interface(), nil
		}
		return nil, nil
	}

	// last resort, return nil
	return nil, nil
}

// This method looks up the field on the given type definition.
// It has special casing for the two introspection fields, __schema
// and __typename. __typename is special because it can always be
// queried as a field, even in situations where no other fields
// are allowed, like on a Union. __schema could get automatically
// added to the query type, but that would require mutating type
// definitions, which would cause issues.
func getFieldDef(schema Schema, parentType *Object, fieldName string, disallowIntrospection bool) *FieldDefinition {
	if parentType == nil {
		return nil
	}

	if !disallowIntrospection {
		if fieldName == SchemaMetaFieldDef.Name &&
			schema.QueryType() == parentType {
			return SchemaMetaFieldDef
		}
		if fieldName == TypeMetaFieldDef.Name &&
			schema.QueryType() == parentType {
			return TypeMetaFieldDef
		}
	}
	if fieldName == TypeNameMetaFieldDef.Name {
		return TypeNameMetaFieldDef
	}
	return parentType.Fields()[fieldName]
}
