package validator

import (
	"errors"
	"fmt"

	"github.com/bucketd/go-graphqlparser/ast"
)

// executableDefinitions ...
func executableDefinitions(vctx *validationContext, walker *ast.Walker) {
	walker.AddDefinitionEventHandler(ast.WalkerEnter, func(def ast.Definition) {
		if def.Kind != ast.DefinitionKindExecutable {
			vctx.errors = vctx.errors.Add(errors.New(
				nonExecutableDefinitionMessage(def),
			))
		}
	})
}

// nonExecutableDefinitionMessage ...
func nonExecutableDefinitionMessage(def ast.Definition) string {
	var name string

	// TODO(elliot): We really need to move the name field to the top level of ast.Definition...
	switch def.Kind {
	case ast.DefinitionKindTypeSystem:
		tsDef := def.TypeSystemDefinition

		switch tsDef.Kind {
		case ast.TypeSystemDefinitionKindSchema:
			name = "schema"
		case ast.TypeSystemDefinitionKindType:
			name = tsDef.TypeDefinition.Name
		case ast.TypeSystemDefinitionKindDirective:
			name = tsDef.DirectiveDefinition.Name
		}
	case ast.DefinitionKindTypeSystemExtension:
		tseDef := def.TypeSystemExtension

		switch tseDef.Kind {
		case ast.TypeSystemExtensionKindSchema:
			name = "schema"
		case ast.TypeSystemExtensionKindType:
			name = tseDef.TypeExtension.Name
		}
	}

	return fmt.Sprintf("The %s definition is not executable.", name)
}
