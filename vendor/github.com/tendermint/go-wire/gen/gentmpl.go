package gen

import (
	"strings"

	"github.com/clipperhouse/typewriter"
)

var templates = typewriter.TemplateSlice{
	wrapper,
	register,
}

// this is the template for generating the go-data wrappers of an interface
var wrapper = &typewriter.Template{
	Name:           "Wrapper",
	TypeConstraint: typewriter.Constraint{},
	FuncMap:        fmap,
	Text: `
type {{.Wrapper}} struct {
  {{.Inner}} "json:\"unwrap\""
}

var {{.Wrapper}}Mapper = data.NewMapper({{.Wrapper}}{})

func (h {{.Wrapper}}) MarshalJSON() ([]byte, error) {
  return {{.Wrapper}}Mapper.ToJSON(h.{{.Inner}})
}

func (h *{{.Wrapper}}) UnmarshalJSON(data []byte) (err error) {
  parsed, err := {{.Wrapper}}Mapper.FromJSON(data)
  if err == nil && parsed != nil {
    h.{{.Inner}} = parsed.({{.Inner}})
  }
  return err
}

// Unwrap recovers the concrete interface safely (regardless of levels of embeds)
func (h {{.Wrapper}}) Unwrap() {{.Inner}} {
  hi := h.{{.Inner}}
  for wrap, ok := hi.({{.Wrapper}}); ok; wrap, ok = hi.({{.Wrapper}}) {
    hi = wrap.{{.Inner}}
  }
  return hi
}

func (h {{.Wrapper}}) Empty() bool {
  return h.{{.Inner}} == nil
}

/*** below are bindings for each implementation ***/
`,
}

var register = &typewriter.Template{
	Name:           "Register",
	TypeConstraint: typewriter.Constraint{},
	FuncMap:        fmap,
	Text: `
func init() {
  {{.Wrapper}}Mapper.RegisterImplementation({{ if .Impl.Pointer }}&{{ end }}{{.Impl.Name}}{}, "{{.ImplType }}", 0x{{.Count}})
}

func (hi {{ if .Impl.Pointer }}*{{ end }}{{.Impl.Name}}) Wrap() {{.Wrapper}} {
  return {{.Wrapper}}{hi}
}
`,
}

var fmap = map[string]interface{}{
	"ToLower": strings.ToLower,
}
