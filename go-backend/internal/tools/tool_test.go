package tools

import (
	"encoding/json"
	"testing"
)

func TestObjectSchemaOmitsEmptyRequired(t *testing.T) {
	data, err := json.Marshal(objectSchema(map[string]any{
		"path": map[string]any{"type": "string"},
	}))
	if err != nil {
		t.Fatal(err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema["required"]; ok {
		t.Fatalf("expected required to be omitted when empty, got %s", data)
	}
}

func TestObjectSchemaIncludesRequiredArray(t *testing.T) {
	data, err := json.Marshal(objectSchema(map[string]any{
		"path": map[string]any{"type": "string"},
	}, "path"))
	if err != nil {
		t.Fatal(err)
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "path" {
		t.Fatalf("expected required path, got %s", data)
	}
}
