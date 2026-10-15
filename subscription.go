package graphql

import (
	"context"
	"fmt"
	"time"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
)

// SubscribeParams configures Subscribe. The execution fields mirror ExecuteParams and are
// applied to every event execution.
type SubscribeParams struct {
	Schema                          Schema
	Root                            any
	AST                             *ast.Document
	OperationName                   string
	Args                            map[string]any
	DeprecatedFieldFn               func(ctx context.Context, parent *Object, fieldDef *FieldDefinition) error
	FieldDefinitionDirectiveHandler func(context.Context, *ast.Directive, *FieldDefinition) error
	DisallowIntrospection           bool
	EnableCoroutines                bool
	TimeoutWait                     time.Duration
	Tracer                          Tracer
	// EventContext derives the context used to execute one event from the subscription
	// context. It is where a caller attaches per-event deadlines and request-scoped caches.
	// The returned cancel func is called after the event's result has been delivered.
	EventContext func(ctx context.Context) (context.Context, context.CancelFunc)
}

// Subscribe creates the source event stream for a subscription operation and returns the
// response stream. It parses nothing: the document must already be validated. Errors
// raised while creating the source stream (unknown operation, more than one root field,
// a field without a Subscribe function, argument coercion, the field's Subscribe call) are
// request errors and are returned directly. Once the stream exists every event is executed
// as the root value of the operation's selection set, exactly like a query, and the result
// is sent on the returned channel. The channel is closed when the source channel closes or
// ctx is done. Sends never block past ctx, so a consumer that stops reading only needs to
// cancel ctx.
func Subscribe(ctx context.Context, p SubscribeParams) (<-chan *Result, error) {
	exeContext, err := buildExecutionContext(BuildExecutionCtxParams{
		Schema:                          p.Schema,
		Root:                            p.Root,
		AST:                             p.AST,
		OperationName:                   p.OperationName,
		Args:                            p.Args,
		DeprecatedFieldFn:               p.DeprecatedFieldFn,
		FieldDefinitionDirectiveHandler: p.FieldDefinitionDirectiveHandler,
		DisallowIntrospection:           p.DisallowIntrospection,
		Tracer:                          p.Tracer,
		EnableCoroutines:                p.EnableCoroutines,
	})
	if err != nil {
		return nil, gqlerrors.FormatError(err)
	}
	if exeContext.Operation.GetOperation() != ast.OperationTypeSubscription {
		return nil, newSubscribeError("Subscribe can only execute subscription operations", exeContext.Operation)
	}
	subscriptionType, err := getOperationRootType(p.Schema, exeContext.Operation)
	if err != nil {
		return nil, gqlerrors.FormatError(err)
	}

	fields, _ := collectFields(CollectFieldsParams{
		ExeContext:   exeContext,
		RuntimeType:  subscriptionType,
		SelectionSet: exeContext.Operation.GetSelectionSet(),
	})
	if len(fields) != 1 {
		return nil, newSubscribeError("Subscription operations must have exactly one root field", exeContext.Operation)
	}
	var fieldASTs []*ast.Field
	for _, f := range fields {
		fieldASTs = f
	}
	fieldAST := fieldASTs[0]
	var fieldName string
	if fieldAST.Name != nil {
		fieldName = fieldAST.Name.Value
	}
	fieldDef := getFieldDef(p.Schema, subscriptionType, fieldName, p.DisallowIntrospection)
	if fieldDef == nil {
		return nil, newSubscribeError(fmt.Sprintf("Unknown subscription field %q", fieldName), fieldAST)
	}
	if fieldDef.Subscribe == nil {
		return nil, newSubscribeError(fmt.Sprintf("Subscription field %q does not provide an event stream", fieldName), fieldAST)
	}
	if fieldDef.DeprecationReason != "" && p.DeprecatedFieldFn != nil {
		if err := p.DeprecatedFieldFn(ctx, subscriptionType, fieldDef); err != nil {
			return nil, gqlerrors.FormatError(err)
		}
	}
	if p.FieldDefinitionDirectiveHandler != nil {
		for _, d := range fieldDef.Directives {
			if err := p.FieldDefinitionDirectiveHandler(ctx, d, fieldDef); err != nil {
				return nil, gqlerrors.FormatError(err)
			}
		}
	}
	args, err := getArgumentValues(fieldDef.Args, fieldAST.Arguments, exeContext.VariableValues)
	if err != nil {
		return nil, gqlerrors.FormatError(err)
	}
	params := ResolveParams{
		Source: p.Root,
		Args:   args,
		Info: ResolveInfo{
			FieldName:      fieldName,
			FieldASTs:      fieldASTs,
			ReturnType:     fieldDef.Type,
			ParentType:     subscriptionType,
			Schema:         p.Schema,
			Fragments:      exeContext.Fragments,
			RootValue:      p.Root,
			Operation:      exeContext.Operation,
			VariableValues: exeContext.VariableValues,
		},
	}

	source, err := callSubscribe(ctx, fieldDef.Subscribe, fieldASTs, params)
	if err != nil {
		return nil, gqlerrors.FormatError(err)
	}
	if source == nil {
		return nil, newSubscribeError(fmt.Sprintf("Subscription field %q returned no event stream", fieldName), fieldAST)
	}

	eventContext := p.EventContext
	if eventContext == nil {
		eventContext = func(ctx context.Context) (context.Context, context.CancelFunc) { return ctx, func() {} }
	}
	executeParams := ExecuteParams{
		Schema:                          p.Schema,
		AST:                             p.AST,
		OperationName:                   p.OperationName,
		Args:                            p.Args,
		DeprecatedFieldFn:               p.DeprecatedFieldFn,
		FieldDefinitionDirectiveHandler: p.FieldDefinitionDirectiveHandler,
		DisallowIntrospection:           p.DisallowIntrospection,
		EnableCoroutines:                p.EnableCoroutines,
		TimeoutWait:                     p.TimeoutWait,
		Tracer:                          p.Tracer,
	}

	out := make(chan *Result)
	go func() {
		defer close(out)
		for {
			var event any
			var ok bool
			select {
			case <-ctx.Done():
				return
			case event, ok = <-source:
				if !ok {
					return
				}
			}
			result := executeEvent(ctx, eventContext, executeParams, event)
			select {
			case <-ctx.Done():
				return
			case out <- result:
			}
		}
	}()
	return out, nil
}

// executeEvent runs the operation's selection set with the event as the root value. The
// event context is cancelled only after the result has been fully built.
func executeEvent(ctx context.Context, eventContext func(context.Context) (context.Context, context.CancelFunc), p ExecuteParams, event any) *Result {
	eventCtx, cancel := eventContext(ctx)
	defer cancel()
	p.Root = event
	return Execute(eventCtx, p)
}

// callSubscribe invokes the field's Subscribe function, converting a panic into an error so
// a misbehaving producer fails the request rather than the process.
func callSubscribe(ctx context.Context, subscribeFn FieldSubscribeFn, fieldASTs []*ast.Field, params ResolveParams) (source <-chan any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recoverFieldPanic(r, fieldASTs)
		}
	}()
	return subscribeFn(ctx, params)
}

func newSubscribeError(message string, node ast.Node) error {
	return gqlerrors.NewError(
		gqlerrors.ErrorTypeBadQuery,
		message,
		[]ast.Node{node},
		"",
		nil,
		[]int{},
		nil,
	)
}

// EventStream adapts a typed event channel to the untyped channel a FieldSubscribeFn
// returns. The forwarding goroutine exits when ctx is done or in closes, so a typed
// producer only needs to honour ctx itself.
func EventStream[T any](ctx context.Context, in <-chan T) <-chan any {
	out := make(chan any)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}
	}()
	return out
}
