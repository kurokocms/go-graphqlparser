package validation

import (
	"fmt"
	"testing"

	"github.com/bucketd/go-graphqlparser/ast"
	"github.com/bucketd/go-graphqlparser/language"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proxyT ...
type proxyT struct {
	name  string
	realT assert.TestingT
}

// newProxyT ...
func newProxyT(t assert.TestingT, name string) *proxyT {
	return &proxyT{
		name:  name,
		realT: t,
	}
}

// Errorf ...
func (t *proxyT) Errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("%s: %s", t.name, format)
	t.realT.Errorf(format, args...)
}

func TestNewWalker(t *testing.T) {
	t.Run("should not return nil", func(t *testing.T) {
		assert.NotNil(t, NewWalker(nil))
	})
}

func TestWalker_Walk(t *testing.T) {
	var pt assert.TestingT

	tt := []struct {
		name     string
		query    []byte
		visitFns VisitFunc
	}{
		{
			name:  "simple selection",
			query: []byte(`{ hello }`),
			visitFns: func(w *Walker) {
				w.AddSelectionEnterEventHandler(func(selection ast.Selection) {
					assert.Equal(pt, "hello", selection.Name)
				})
			},
		},
		{
			name: "simple type system",
			query: []byte(`
				type Climbing implements
						& Ropes
						& Rocks
						& Chalk {
					name: String
					weight: Int
				}
			`),
			visitFns: func(w *Walker) {
				var count int
				w.AddTypeEnterEventHandler(func(gt ast.Type) {
					count++
				})

				w.AddDocumentLeaveEventHandler(func(document ast.Document) {
					assert.Equal(pt, 5, count)
				})
			},
		},
		{
			name: "deeply nested directive",
			query: []byte(`
				{ foo { bar { baz { qux { quux { corge { uier { grault @garply } } } } } } } }
			`),
			visitFns: func(w *Walker) {
				w.AddDirectiveEnterEventHandler(func(directive ast.Directive) {
					assert.Equal(pt, "garply", directive.Name)
				})
			},
		},
		{
			name: "item within list value",
			query: []byte(`
				{
					foo(list: ["bar"])
				}
			`),
			visitFns: func(w *Walker) {
				w.AddStringValueEnterEventHandler(func(value ast.Value) {
					assert.Equal(pt, "bar", value.StringValue)
				})
			},
		},
		{
			name: "deeply nested object values",
			query: []byte(`
				mutation {
					createMessage(input: {
						author: {
							name: "seer",
						},
						content: "hope is a good thing",
					}) {
						id
					}
				}
			`),
			visitFns: func(w *Walker) {
				var authorName string
				w.AddObjectFieldEnterEventHandler(func(field ast.ObjectField) {
					if field.Name == "name" {
						authorName = field.Value.StringValue
					}
				})

				w.AddDocumentLeaveEventHandler(func(document ast.Document) {
					assert.Equal(pt, "seer", authorName)
				})
			},
		},
		{
			name: "not walking pointer fields that aren't set",
			query: []byte(`
				query {
					hello
				}
			`),
			visitFns: func(w *Walker) {
				w.AddDirectiveEnterEventHandler(func(directive ast.Directive) {
					assert.Fail(pt, "we shouldn't have walked directives, when none exist")
				})
			},
		},
		{
			name: "correct kind walk function(s) used",
			query: []byte(`
				mutation CreateFoo($foo: int) {
					createFoo(foo: $foo) {
						name
					}
				}
			`),
			visitFns: func(w *Walker) {
				var calls int
				w.AddMutationOperationDefinitionEnterEventHandler(func(handler *ast.OperationDefinition) {
					calls++
				})

				w.AddQueryOperationDefinitionEnterEventHandler(func(definition *ast.OperationDefinition) {
					calls++
				})

				w.AddDocumentLeaveEventHandler(func(document ast.Document) {
					assert.Equal(pt, 1, calls)
				})
			},
		},
	}

	for _, tc := range tt {
		pt = newProxyT(t, tc.name)

		parser := language.NewParser(tc.query)

		doc, err := parser.Parse()
		require.NoError(t, err)

		walker := NewWalker([]VisitFunc{tc.visitFns})
		walker.Walk(doc)
	}
}
