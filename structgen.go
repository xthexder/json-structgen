package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

var GlobalTypes map[string]string

type JsonSchema struct {
	Schema string `json:"$schema"`
	Ref    string `json:"$ref"`

	Title                string                 `json:"title"`
	Type                 interface{}            `json:"type"`
	Description          string                 `json:"description"`
	Extends              *JsonSchema            `json:"extends"`
	Properties           map[string]*JsonSchema `json:"properties"`
	AdditionalProperties interface{}            `json:"additionalProperties"`
	Items                *JsonSchema            `json:"items"`
}

func SchemaFromInterface(in interface{}) *JsonSchema {
	if in == nil {
		return nil
	}

	switch in := in.(type) {
	case bool:
		return nil
	case map[string]interface{}:
		out := &JsonSchema{
			Type:    in["type"],
			Extends: SchemaFromInterface(in["extends"]),
			Items:   SchemaFromInterface(in["items"]),
		}
		if str, ok := in["$ref"]; ok {
			out.Ref = str.(string)
		}
		if str, ok := in["title"]; ok {
			out.Title = str.(string)
		}
		if str, ok := in["description"]; ok {
			out.Description = str.(string)
		}
		if prop, ok := in["properties"]; ok {
			out.Properties = make(map[string]*JsonSchema)
			for k, v := range prop.(map[string]interface{}) {
				out.Properties[k] = SchemaFromInterface(v)
			}
		}
		return out
	default:
		panic(fmt.Sprintf("Unknown schema interface: %+v", in))
	}
}

func (js *JsonSchema) GoType(collapse bool) string {
	js.LoadRef()

	switch t := js.Type.(type) {
	case string:
		switch t {
		case "any":
			return "interface{}"
		case "boolean":
			return "bool"
		case "integer":
			return "int64"
		case "number":
			return "float64"
		case "string":
			return "string"
		case "array":
			if js.Items == nil {
				panic(fmt.Sprintf("Schema %+v does not have an array type.", js))
			}
			return "[]" + js.Items.GoType(true)
		case "object":
			name := Capitalize(js.Title)

			if len(js.Properties) == 0 && js.AdditionalProperties != nil {
				return "interface{}"
			}
			src := "struct {\n"
			for _, n := range SortedKeys(js.Properties) {
				src += Capitalize(n) + " " + js.Properties[n].GoType(true) + " `json:\"" + n + "\"`\n"
			}
			src += "}"

			GlobalTypes[name] = src
			if collapse {
				return name
			}
			return src
		default:
			panic("Unknown type string: " + t)
		}
	case []interface{}:
		if len(t) != 1 {
			return "interface{}"
		}
		return (&JsonSchema{Title: js.Title, Type: t[0]}).GoType(collapse)
	default:
		panic(fmt.Sprintf("Unknown type: %+v", js.Type))
	}
}

func (js *JsonSchema) LoadRef() {
	if len(js.Ref) > 0 {
		ref := js.Ref
		js.Ref = ""
		LoadRef(ref, js)
	}
	if len(js.Ref) > 0 {
		panic(fmt.Sprintf("Schema %+v references a schema with a ref.", js))
	}
	if js.Properties == nil {
		js.Properties = make(map[string]*JsonSchema)
	}

	additionalProperties := SchemaFromInterface(js.AdditionalProperties)
	for _, add := range []*JsonSchema{additionalProperties, js.Extends} {
		if add != nil {
			add.LoadRef()

			if len(js.Title) == 0 {
				js.Title = add.Title
			}
			if js.Type == nil {
				js.Type = add.Type
			}
			if js.Items == nil {
				js.Items = add.Items
			}
			for k, v := range add.Properties {
				if _, ok := js.Properties[k]; !ok {
					js.Properties[k] = v
				}
			}
		}
	}
}

func LoadRef(ref string, schema *JsonSchema) {
	file, err := ioutil.ReadFile(ref)
	if err != nil {
		panic("Ref not found: " + ref)
	}

	err = json.Unmarshal(file, schema)
	if err != nil {
		panic(fmt.Sprintf("%s %+v", file, err))
	}
}

func SortedKeys(in interface{}) []string {
	keysReflect := reflect.ValueOf(in).MapKeys()
	keys := make([]string, len(keysReflect))
	for i, k := range keysReflect {
		keys[i] = k.String()
	}
	sort.Strings(keys)
	return keys
}

func Capitalize(in string) (out string) {
	words := strings.Split(in, " ")
	for _, word := range words {
		if len(word) > 0 {
			out += strings.ToUpper(word[0:1]) + word[1:]
		}
	}
	return
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:", os.Args[0], "struct.schema.json [package]")
		os.Exit(1)
		return
	}

	os.Chdir(filepath.Dir(os.Args[1]))

	GlobalTypes = make(map[string]string)

	var schema JsonSchema
	LoadRef(filepath.Base(os.Args[1]), &schema)

	schema.GoType(true)

	if len(os.Args) > 2 {
		fmt.Println("package", os.Args[2])
		fmt.Println()
	}

	for _, name := range SortedKeys(GlobalTypes) {
		src := "type " + name + " " + GlobalTypes[name]
		srcFmt, _ := format.Source([]byte(src))
		fmt.Println(string(srcFmt))
		fmt.Println()
	}
}
