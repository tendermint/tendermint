package callgraph

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"

	"golang.org/x/tools/go/loader"
)

type Triple struct {
	Caller Entry
	Target Entry
	Site   Location
}

type Entry struct {
	Package string // canonical import path
	Name    string // name relative to the package ("" for calls at file scope)
}

type Location struct {
	Path      string
	Offset    int // 0-based
	Line, Col int // 1-based line, 0-based byte offset
}

type Graph struct {
	cfg *loader.Config
}

func New() *Graph {
	cfg := new(loader.Config)
	cfg.TypeCheckFuncBodies = func(ip string) bool {
		_, ok := cfg.ImportPkgs[ip]
		return ok
	}
	return &Graph{cfg: cfg}
}

func (g *Graph) Import(ipath string) { g.cfg.Import(ipath) }

func (g *Graph) ImportWithTests(ipath string) { g.cfg.ImportWithTests(ipath) }

func (g *Graph) Process(f func(*Triple)) error {
	pgm, err := g.cfg.Load()
	if err != nil {
		return fmt.Errorf("loading program: %v", err)
	}
	var pkgs []*loader.PackageInfo
	for _, pkg := range pgm.Imported {
		pkgs = append(pkgs, pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Pkg.Path() < pkgs[j].Pkg.Path()
	})

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			fname := g.cfg.Fset.Position(file.Pos()).Filename

			var nodes []ast.Node
			parent := func() string {
				for i := len(nodes) - 1; i >= 0; i-- {
					switch t := nodes[i].(type) {
					case *ast.FuncDecl:
						return t.Name.Name
					}
				}
				return fname
			}

			ast.Walk(visitFunc(func(node ast.Node) {
				if node == nil {
					nodes = nodes[:len(nodes)-1]
					return
				}
				nodes = append(nodes, node)

				switch t := node.(type) {
				case *ast.Ident:
					ref := pkg.Info.Uses[t]
					if ref == nil {
						return // no referent
					}
					var refPath string
					if _, ok := ref.Type().(*types.Signature); ok {
						refPath = ref.Pkg().Path() // OK, function
					} else if _, ok := ref.(*types.Builtin); ok {
						// OK, builtin
					} else {
						return // not a function call or reference
					}
					pos := g.cfg.Fset.Position(t.Pos())
					f(&Triple{
						Caller: Entry{Package: pkg.Pkg.Path(), Name: parent()},
						Target: Entry{Package: refPath, Name: ref.Name()},
						Site: Location{
							Path:   pos.Filename,
							Offset: pos.Offset,
							Line:   pos.Line,
							Col:    pos.Column - 1,
						},
					})
				}
			}), file)
		}
	}
	return nil
}

type visitFunc func(ast.Node)

func (v visitFunc) Visit(n ast.Node) ast.Visitor { v(n); return v }
