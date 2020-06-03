package json

import (
	"fmt"
	"reflect"
	"strings"
)

// structInfo contains JSON info for a struct.
type structInfo struct {
	fields []*fieldInfo
}

// fieldInfo contains JSON info for a struct field.
type fieldInfo struct {
	jsonName  string
	omitEmpty bool
}

// makeStructInfo generates structInfo for a struct as a reflect.Value.
// FIXME This should be cached.
func makeStructInfo(rt reflect.Type) *structInfo {
	if rt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("can't make struct info for non-struct value %v", rt.String()))
	}
	fields := make([]*fieldInfo, 0, rt.NumField())
	for i := 0; i < cap(fields); i++ {
		frt := rt.Field(i)
		fInfo := &fieldInfo{
			jsonName:  frt.Name,
			omitEmpty: false,
		}
		if o := frt.Tag.Get("json"); o != "" {
			opts := strings.Split(o, ",")
			if opts[0] != "" {
				fInfo.jsonName = opts[0]
			}
			for _, o := range opts[1:] {
				if o == "omitempty" {
					fInfo.omitEmpty = true
				}
			}
		}
		fields = append(fields, fInfo)
	}

	return &structInfo{fields: fields}
}
