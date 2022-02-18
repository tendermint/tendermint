// Package bicall locates calls of built-in functions in Go source.
package bicall

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
)

// See https://golang.org/ref/spec#Built-in_functions
var isBuiltin = map[string]bool{
	"append":  true,
	"cap":     true,
	"close":   true,
	"complex": true,
	"copy":    true,
	"delete":  true,
	"imag":    true,
	"len":     true,
	"make":    true,
	"new":     true,
	"panic":   true,
	"print":   true,
	"println": true,
	"real":    true,
	"recover": true,
}

// Call represents a call to a built-in function in a source program.
type Call struct {
	Name     string         // the name of the built-in function
	Call     *ast.CallExpr  // the call expression in the AST
	Site     token.Position // the location of the call
	Path     []ast.Node     // the AST path to the call
	Comments []string       // comments attributed to the call site by the parser
}

// flattenComments extracts the text of the given comment groups, and removes
// leading and trailing whitespace from them.
func flattenComments(cgs []*ast.CommentGroup) (out []string) {
	for _, cg := range cgs {
		out = append(out, strings.TrimSuffix(cg.Text(), "\n"))
	}
	return
}

// Parse parses the contents of r as a Go source file, and calls f for each
// call expression targeting a built-in function that occurs in the resulting
// AST. Location information about each call is attributed to the specified
// filename.
//
// If f reports an error, traversal stops and that error is reported to the
// caller of Parse.
func Parse(r io.Reader, filename string, f func(Call) error) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, r, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", filename, err)
	}
	cmap := ast.NewCommentMap(fset, file, file.Comments)

	var path []ast.Node

	// Find the comments associated with node. If the node does not have its own
	// comments, scan upward for a statement containing the node.
	commentsFor := func(node ast.Node) []string {
		if cgs := cmap[node]; cgs != nil {
			return flattenComments(cgs)
		}
		for i := len(path) - 2; i >= 0; i-- {
			if _, ok := path[i].(ast.Stmt); !ok {
				continue
			}
			if cgs := cmap[path[i]]; cgs != nil {
				return flattenComments(cgs)
			} else {
				break
			}
		}
		return nil
	}
	v := &visitor{
		visit: func(node ast.Node) error {
			if node == nil {
				path = path[:len(path)-1]
				return nil
			}
			path = append(path, node)
			if call, ok := node.(*ast.CallExpr); ok {
				id, ok := call.Fun.(*ast.Ident)
				if !ok || !isBuiltin[id.Name] {
					return nil
				}

				if err := f(Call{
					Name:     id.Name,
					Call:     call,
					Site:     fset.Position(call.Pos()),
					Path:     path,
					Comments: commentsFor(node),
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	ast.Walk(v, file)
	return v.err
}

type visitor struct {
	err   error
	visit func(ast.Node) error
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if v.err == nil {
		v.err = v.visit(node)
	}
	if v.err != nil {
		return nil
	}
	return v
}
