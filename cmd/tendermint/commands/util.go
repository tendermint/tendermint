package commands

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
)

// indexTaggedStructFields walks through a struct, looking up those
// fields tagged by the requested tagName recursively flattening
// anonymous struct fields by calling indexTaggedStructFields
// on them, only if the ",squash" sub-tag is applied to the fields.
// For example, given:
//  type A struct {
//    Foo int `json:"foo"`
//    Bar string `mapstructure:"bar"`
//    Baz string `mapstructure:"baz"`
//    B   *A `mapstructure:"b,squash"`
//    C   *A `mapstructure:"c"`
//  }
//
//  &A{Foo: 1, Bar: "1", Baz: "B", B: &A{Foo: 2, Bar: "2"}, "C": &A{Foo: 3}}
//
// It produces:
//
//  {
//    "foo": 1, "bar": "1", "baz": "B",
//    "b": map[string]interface{}{"foo": 2, "bar": "2"}
//  }
func indexTaggedStructFields(v interface{}, tagName string) map[string]interface{} {
	val := reflect.ValueOf(v)
	fieldsMap := make(map[string]interface{})
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}

	typ := val.Type()
	n := val.NumField()

	for i := 0; i < n; i++ {
		fVal := val.Field(i)
		field := typ.Field(i)
		tag, _, squash := extractTag(tagName, field)
		if tag != "" {
			fieldsMap[tag] = fVal.Interface()
		} else if squash {
			innerMap := indexTaggedStructFields(fVal.Interface(), tagName)
			// Since this is an anonymous struct,
			// we should squash as requested it's content inside
			for k, v := range innerMap {
				if _, alreadyIn := fieldsMap[k]; !alreadyIn {
					fieldsMap[k] = v
				}
			}
		}
	}
	return fieldsMap
}

func extractTag(tagName string, f reflect.StructField) (theTagName string, omitEmpty, squash bool) {
	tag := f.Tag.Get(tagName)
	splits := strings.Split(tag, ",")
	if len(splits) == 0 {
		return "", false, false
	}
	if len(splits) > 1 {
		switch splits[1] {
		case "omitempty":
			omitEmpty = true
		case "squash":
			squash = true
		}
	}
	return splits[0], omitEmpty, squash
}

// filterOutUnknownFields tallies fields
// retrieved by parsing out fields in want
// with those in gotFieldsMap and
// reporting an error if there are fields in
// gotFieldsMap that don't match up.
//
// A usecase is to report fields that were set in
// the config file for a command but are unexpected.
func filterOutUnknownFields(want interface{}, gotFieldsMap map[string]interface{}, tagName string, excusedFlags map[string]map[string]bool) error {
	wantFieldsMap := indexTaggedStructFields(want, tagName)
	unpermittedFields := make(map[string][]string)
	for key, value := range gotFieldsMap {
		innerMap, specializedCommand := value.(map[string]interface{})
		// Specialized commands have data in the form:
		//  "p2p": {"laddr": "tcp://0.0.0.1"}
		//
		// originating from the config file as:
		//  [p2p]
		//  laddr = "tcp://0.0.0.1"
		//
		// whereas flags in the global scope
		// are key value pairs of the form:
		//  "laddr": "tcp://0.0.01"
		// originating from the config file as:
		//  laddr = "tcp://0.0.0.1"
		inGlobalScope := !specializedCommand
		if inGlobalScope {
			_, permitted := wantFieldsMap[key]
			if !permitted {
				unpermittedFields[globalScope] = append(unpermittedFields[globalScope], key)
			}
			continue
		}

		// Otherwise we are in a specialized command
		// Whose data looks like this: "p2p": {"laddr": "tcp://0.0.0.1"}
		wantCfg := wantFieldsMap[key]
		wantKeyFieldsMap := indexTaggedStructFields(wantCfg, tagName)
		for kkey, _ := range innerMap {
			// Skip zero values. Note that we aren't skipping ""
			// despite it being a zero value, because
			switch value {
			case false, nil, 0:
				continue
			}

			_, permitted := wantKeyFieldsMap[kkey]
			if !permitted {
				unpermittedFields[key] = append(unpermittedFields[key], kkey)
			}
		}
	}

	if len(unpermittedFields) == 0 {
		return nil
	}

	filteredMapFlags := make(map[string][]string)
	for cmdScope, flags := range unpermittedFields {
		if cmdScope == "" {
			cmdScope = globalScope
		}
		excusedMap := excusedFlags[cmdScope]
		filteredFlags := make([]string, 0, len(flags))
		for _, flag := range flags {
			if _, excused := excusedMap[flag]; !excused {
				filteredFlags = append(filteredFlags, flag)
			}
		}
		if len(filteredFlags) == 0 {
			continue
		}
		filteredMapFlags[cmdScope] = filteredFlags
	}

	if len(filteredMapFlags) == 0 {
		return nil
	}

	// We should deterministically print out error
	// outputs hence the sorting of scopes first here.
	scopeKeys := make([]string, 0, len(filteredMapFlags))
	for scope := range filteredMapFlags {
		scopeKeys = append(scopeKeys, scope)
	}
	sort.Strings(scopeKeys)

	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "We've detected these unknown flags in your config file\n")
	for _, scope := range scopeKeys {
		flags := filteredMapFlags[scope]
		fmt.Fprintf(buf, "%s:\n", scope)

		fflags := make([]string, len(flags))
		copy(fflags, flags)
		// We should deterministically generate output
		// errors within a scope hence the sorting here.
		sort.Strings(fflags)
		for _, flag := range fflags {
			fmt.Fprintf(buf, "\t%s\n", flag)
		}
	}
	return fmt.Errorf("%s", buf.Bytes())
}

const (
	globalScope = "global"
	verifyFlag  = "check-config"
)

// excusedFlags contains flags that not set inside
// the struct but those that are used by the global
// config or passed on the commandline.
var excusedFlagsMap = map[string]map[string]bool{
	globalScope: {
		"root":     true,
		"help":     true,
		"trace":    true,
		verifyFlag: true,
	},
}

// handleConfigVerification firstly checks if the flag `--verify` was set
// and if not, returns the passed in config unfiltered and unchecked
// otherwise it tallies up the values in the config file with the fields
// of the config struct itself.
// See https://github.com/tendermint/tendermint/issues/628.
func handleConfigVerification(conf *cfg.Config) (*cfg.Config, error) {
	// TODO: Move this step into viper and push up stream.
	if !viper.GetBool(verifyFlag) {
		return conf, nil
	}
	gotFieldsMap := viper.AllSettings()
	err := filterOutUnknownFields(conf, gotFieldsMap, "mapstructure", excusedFlagsMap)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
