package graphql

import "github.com/bucketd/go-graphqlparser/ast"

// Schema ...
type Schema struct {
	// NOTE: We can't really include this, because we may re-use a Schema instance to extend an
	// existing schema, e.g. if we were stitching together multiple schema documents. One
	// alternative would be to keep a slice / map of documents instead - if we needed them. We
	// should aim to store enough information on this type that we don't need the original
	// ast.Document for the schema.
	//Document ast.Document

	Definition *ast.SchemaDefinition

	// NOTE: The following should all have the "kind" ast.TypeKindNamed.
	QueryType        *ast.Type
	MutationType     *ast.Type
	SubscriptionType *ast.Type

	Directives map[string]*ast.DirectiveDefinition
	Types      map[string]*ast.TypeDefinition
}
