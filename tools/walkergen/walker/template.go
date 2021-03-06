package walker

import (
	"fmt"
	"io"
	"text/template"

	"github.com/Masterminds/sprig"
)

// header is a comment placed at the top of the file to signal to other Go tools, and users to not
// alter this file, as changes may be overwritten anyway.
var header = `
// Code generated by lab/walkergen
// DO NOT EDIT!
`

// imports specifies the list of imports for when we generate a walker outside of our ast package.
var imports = `
import "github.com/bucketd/go-graphqlparser/ast"
`

// walkerTypeTmpl is the template used to generate the type declaration, and constructor for the
// Walker type, populating it with event handlers as fields.
var walkerTypeTmpl = template.Must(template.New("walkerTypeTmpl").Funcs(sprig.TxtFuncMap()).Parse(`
// Walker holds event handlers for entering and leaving AST nodes.
type Walker struct { {{- range .}}
	{{untitle .FuncName}}EventHandlers {{.FuncName}}EventHandlers{{end}}
}

// NewWalker returns a new Walker instance.
func NewWalker(visitFns []VisitFunc) *Walker {
	walker := &Walker{}
	for _, visitFn := range visitFns {
		visitFn(walker)
	}

	return walker
}

// Walk traverses an entire AST document for the purposes of validation.
func (w *Walker) Walk(ctx *Context, doc ast.Document) {
	w.walkDocument(ctx, doc)
}
`))

// eventHandlersTmpl is the template used to generate the event handler functions for the walker,
// and the corresponding types that have the event handler functions attached.
var eventHandlersTmpl = template.Must(template.New("eventHandlersTmpl").Funcs(sprig.TxtFuncMap()).Parse(`
// {{.FuncName}}EventHandler function can handle enter/leave events for {{.FuncName}}.
type {{.FuncName}}EventHandler func(*Context, {{if .IsAlwaysPointer}}*{{end}}ast.{{.TypeName}})

// {{.FuncName}}EventHandlers stores the enter and leave events handlers.
type {{.FuncName}}EventHandlers struct {
	enter []{{.FuncName}}EventHandler
	leave []{{.FuncName}}EventHandler
}

// Add{{.FuncName}}EnterEventHandler adds an event handler to be called when entering {{.FuncName}} nodes.
func (w *Walker) Add{{.FuncName}}EnterEventHandler(h {{.FuncName}}EventHandler) {
	w.{{untitle .FuncName}}EventHandlers.enter = append(w.{{untitle .FuncName}}EventHandlers.enter, h)
}

// Add{{.FuncName}}LeaveEventHandler adds an event handler to be called when leaving {{.FuncName}} nodes.
func (w *Walker) Add{{.FuncName}}LeaveEventHandler(h {{.FuncName}}EventHandler) {
	w.{{untitle .FuncName}}EventHandlers.leave = append(w.{{untitle .FuncName}}EventHandlers.leave, h)
}

// On{{.FuncName}}Enter calls the enter event handlers registered for this node type.
func (w *Walker) On{{.FuncName}}Enter(ctx *Context, {{.ShortTypeName}} {{if .IsAlwaysPointer}}*{{end}}ast.{{.TypeName}}) {
	for _, handler := range w.{{untitle .FuncName}}EventHandlers.enter {
		handler(ctx, {{.ShortTypeName}})
	}
}

// On{{.FuncName}}Leave calls the leave event handlers registered for this node type.
func (w *Walker) On{{.FuncName}}Leave(ctx *Context, {{.ShortTypeName}} {{if .IsAlwaysPointer}}*{{end}}ast.{{.TypeName}}) {
	for _, handler := range w.{{untitle .FuncName}}EventHandlers.leave {
		handler(ctx, {{.ShortTypeName}})
	}
}
`))

// walkFnHeadTmpl is the template for generating the head of a walker function.
var walkFnHeadTmpl = template.Must(template.New("walkFnHeadTmpl").Funcs(sprig.TxtFuncMap()).Parse(`
// walk{{.FuncName}} is a function that walks {{.FuncName}} type's AST node.
func (w *Walker) walk{{.FuncName}}(ctx *Context, {{.ShortTypeName}} {{.FullTypeName}}) {
	w.On{{.FuncName}}Enter(ctx, {{.ShortTypeName}})
`))

// walkFnFootTmpl is the template for generating the head of a walker function.
var walkFnFootTmpl = template.Must(template.New("walkFnFootTmpl").Funcs(sprig.TxtFuncMap()).Parse(`
	w.On{{.FuncName}}Leave(ctx, {{.ShortTypeName}})
}
`))

var walkFnLinkedListTmpl = template.Must(template.New("walkFnLinkedListTmpl").Parse(`
	{{.ShortTypeName}}.ForEach(func({{.LinkedListType.ShortTypeName}} {{.LinkedListType.FullTypeName}}, i int) {
		w.walk{{.LinkedListType.FuncName}}(ctx, {{.LinkedListType.ShortTypeName}})
	})
`))

var walkFnKindsTmpl = template.Must(template.New("walkFnKindsTmpl").Parse(`
	switch {{.ShortTypeName}}.Kind {
	{{range .Kinds -}}
	case ast.{{.ConstName}}:
		w.walk{{.Type.FuncName}}(ctx, {{$.ShortTypeName}}
			{{- if not .IsSelf -}}
				.{{.Field.Name}}
			{{- end -}}
		)
	{{end -}}
	}
`))

// walkerFnTmpl ...
func walkerFnTmpl(w io.Writer, wt walkerType) error {
	err := walkFnHeadTmpl.Execute(w, wt)
	if err != nil {
		return err
	}

	if wt.IsLinkedList {
		err := walkFnLinkedListTmpl.Execute(w, wt)
		if err != nil {
			return err
		}
	} else if len(wt.Kinds) > 0 {
		err := walkFnKindsTmpl.Execute(w, wt)
		if err != nil {
			return err
		}
	} else if len(wt.Fields) > 0 {
		fmt.Println("")
		for i, fld := range wt.Fields {
			needsDeref := fld.IsPointerType && !fld.Type.IsAlwaysPointer
			needsNilCheck := needsDeref || fld.Type.IsAlwaysPointer

			deref := ""
			if needsDeref {
				deref = "*"
			}

			accessor := ""
			indent := "\t"
			if fld.IsSliceType {
				accessor = "[i]"
				indent += "\t"
				fmt.Fprintf(w, "\tfor i := range %s%s.%s {\n", deref, wt.ShortTypeName, fld.Name)
			}

			if needsNilCheck {
				indent += "\t"
				fmt.Fprintf(w, "\tif %s.%s != nil {\n", wt.ShortTypeName, fld.Name)
			}

			fmt.Fprintf(w, "%sw.walk%s(ctx, %s%s.%s%s)\n", indent, fld.Type.FuncName, deref, wt.ShortTypeName, fld.Name, accessor)

			if needsNilCheck {
				fmt.Fprintf(w, "\t}\n")
			}

			if fld.IsSliceType {
				fmt.Fprintf(w, "\t}\n")
			}

			if i < len(wt.Fields)-1 && (needsNilCheck || fld.IsSliceType) {
				fmt.Fprintf(w, "\n")
			}
		}
	}

	err = walkFnFootTmpl.Execute(w, wt)
	if err != nil {
		return err
	}

	return nil
}

// walkerType holds information about an AST type and is used for generating the walker.
type walkerType struct {
	// TypeName is the type name without any decoration (e.g. without `[]`, `*`, and/or `ast.`)
	TypeName string
	// FuncName is the name used as the name of the walker function.
	FuncName string
	// ShortTypeName is the name that's normally used as a variable name for this type.
	ShortTypeName string
	// FullTypeName is the name with all decoration (i.e. with `[]`, `*`, and/or `ast.`)
	FullTypeName string
	// Fields is a slice of fields of this type.
	Fields []walkerTypeField
	// Kinds is a slice of kinds of this type.
	Kinds []walkerTypeKind
	// LinkedListType is set if this type is a linked list type, and contains the node type
	// information, so that the walk function may be called for the node type.
	LinkedListType *walkerType
	// IsAlwaysPointer is true if this type is only ever used as a pointer in the AST. If the value
	// is always a pointer, we should generate a nil check at the top of the walker, as a pointer
	// value will be passed in. Otherwise, a nil check will be generated elsewhere.
	IsAlwaysPointer bool
	// IsLinkedList is true if this type appears to be a generated linked list type.
	IsLinkedList bool
}

// walkerTypeField holds information about a specific field on an AST type that is pertinent to
// generating part of a walker. Given that walkerTypeFields also have a link to another walkerType,
// they are only used for non-built-in types (i.e. not string, int, etc.)
type walkerTypeField struct {
	// TypeName is the name of this field on the type it belongs to.
	Name string
	// Type holds information about the type that this field refers to.
	Type walkerType
	// OnKinds controls if a field will only be walked for a specific kind.
	OnKinds []string
	// IsPointerType is true if this field's type is a pointer type. If it is a pointer, we should
	// check if it is always a pointer. If it is always a pointer, we can just pass it into the
	// relevant walk function. If not, then we need to do a nil check, then dereference it to pass
	// it to it's walk function.
	IsPointerType bool
	// IsSliceType is true if this field's type is a slice type. If it is a slice type, we should
	// loop over the slice in this walk function, passing each individual item into a walk function.
	// A slice may also be nil, so we should also do a nil check.
	IsSliceType bool

	// isASTType is set to true if the type that this field corresponds to is an AST type, and not
	// a Go built-in type.
	isASTType bool
	// typeName is the name of the type that this field refers to.
	typeName string
}

// walkerTypeKind holds information about the different "Kinds" related to a type.
type walkerTypeKind struct {
	// ConstName is the name of the const used to identify this kind.
	ConstName string
	// Type is the type that corresponds to this kind. Even with a "self" kind, we use this for the
	// walk function name.
	Type walkerType
	// Field is the field that corresponds to this kind. The field may be nil, in the case of a self
	// kind, hence IsSelfKind.
	Field *walkerTypeField
	// IsSelf is true if this "Kind" refers to the type that the Kind field is on. If it is false,
	// then Field should be non-nil.
	IsSelf bool

	// fieldName is the name of the field that this kind kind corresponds to.
	fieldName string
}
