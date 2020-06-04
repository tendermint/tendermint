package json

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	lru "github.com/hashicorp/golang-lru"
)

// structInfoCache caches struct info.
var structInfoCache *lru.Cache

func init() {
	var err error
	structInfoCache, err = lru.New(1000)
	if err != nil {
		panic(err)
	}
}

// structInfo contains JSON info for a struct.
type structInfo struct {
	fields []*fieldInfo
}

// fieldInfo contains JSON info for a struct field.
type fieldInfo struct {
	jsonName  string
	omitEmpty bool
	hidden    bool
}

// makeStructInfo generates structInfo for a struct as a reflect.Value.
func makeStructInfo(rt reflect.Type) *structInfo {
	if rt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("can't make struct info for non-struct value %v", rt))
	}
	if sInfo, ok := structInfoCache.Get(rt); ok {
		return sInfo.(*structInfo)
	}
	fields := make([]*fieldInfo, 0, rt.NumField())
	for i := 0; i < cap(fields); i++ {
		frt := rt.Field(i)
		fInfo := &fieldInfo{
			jsonName:  frt.Name,
			omitEmpty: false,
			hidden:    frt.Name == "" || !unicode.IsUpper(rune(frt.Name[0])),
		}
		o := frt.Tag.Get("json")
		if o == "-" {
			fInfo.hidden = true
		} else if o != "" {
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
	sInfo := &structInfo{fields: fields}
	structInfoCache.Add(rt, sInfo)

	return sInfo
}
