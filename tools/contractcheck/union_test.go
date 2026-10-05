package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnionBackingStorageRequiresSchemaAndCodecs(t *testing.T) {
	t.Parallel()

	baseline := "package wire; import \"encoding/json\"; type Envelope struct {" +
		"Value string `json:\"value\"`; union json.RawMessage }; " +
		"func (t Envelope) MarshalJSON()([]byte,error){b,err:=t.union.MarshalJSON();return b,err};" +
		"func (t *Envelope) UnmarshalJSON(b []byte)error{err:=t.union.UnmarshalJSON(b);return err}"
	for name, source := range map[string]string{
		"generated cache":         baseline,
		"arbitrary private field": strings.Replace(baseline, "union json.RawMessage", "other json.RawMessage", 1),
		"missing codec":           strings.Replace(baseline, "MarshalJSON()([]byte", "Encode()([]byte", 1),
		"exported untagged field": strings.Replace(baseline,
			"union json.RawMessage", "Extra string; union json.RawMessage", 1),
		"wrong cache type": strings.Replace(baseline, "union json.RawMessage", "union string", 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "probe.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}

			group, recognized := file.Decls[1].(*ast.GenDecl)
			if !recognized {
				t.Fatal("model group missing")
			}

			model, recognized := group.Specs[0].(*ast.TypeSpec)
			if !recognized {
				t.Fatal("model type missing")
			}

			structure, recognized := model.Type.(*ast.StructType)
			if !recognized {
				t.Fatal("model structure missing")
			}

			var node yaml.Node

			expected := objectContract{Properties: map[string]yaml.Node{"value": node},
				Required: []string{"value"}, OneOf: []yaml.Node{node, node}}

			err = matchFields(unionWireFields(file, model, structure, expected), expected)
			if (err == nil) != (name == "generated cache") {
				t.Fatalf("cache acceptance differs: %v", err)
			}

			expected.OneOf = nil
			if matchFields(unionWireFields(file, model, structure, expected), expected) == nil {
				t.Fatal("cache accepted without owning oneOf schema")
			}
		})
	}
}
