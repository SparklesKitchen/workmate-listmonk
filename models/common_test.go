package models

import "testing"

func TestJSONScan(t *testing.T) {
	var got JSON
	if err := (&got).Scan([]byte(`{"hosted_form":{"heading":"List form"}}`)); err != nil {
		t.Fatal(err)
	}
	if got["hosted_form"].(map[string]any)["heading"] != "List form" {
		t.Fatalf("JSON scan lost the list attributes: %#v", got)
	}
}
