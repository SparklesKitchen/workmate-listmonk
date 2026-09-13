package main

import (
	"net/url"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestNormalizeSubscriptionFormFields(t *testing.T) {
	form, ok := normalizeSubscriptionForm(models.PublicSubscriptionForm{Fields: []models.PublicSubscriptionFormField{
		{Key: "company", Type: "text", Label: "Company", Required: true},
		{Key: "size", Type: "select", Label: "Team size", Options: []string{"1-10", "11-50"}},
		{Type: "consent", Label: "I agree"},
	}})
	if !ok || len(form.Fields) != 3 {
		t.Fatalf("expected valid ordered fields, got %#v, %v", form.Fields, ok)
	}
	if form.Fields[2].Required != true {
		t.Fatal("consent must always be required")
	}

	_, ok = normalizeSubscriptionForm(models.PublicSubscriptionForm{Fields: []models.PublicSubscriptionFormField{
		{Key: "bad key", Type: "text", Label: "Bad"},
	}})
	if ok {
		t.Fatal("unsafe field key must reject the configured schema")
	}
}

func TestResolveSubscriptionForm(t *testing.T) {
	global := models.PublicSubscriptionForm{Heading: "Global"}
	valid := models.PublicSubscriptionForm{Heading: "List one", Fields: []models.PublicSubscriptionFormField{{Key: "company", Type: "text", Label: "Company"}}}

	tests := []struct {
		name      string
		lists     []models.List
		requested []string
		want      string
	}{
		{
			name:      "uses valid list schema",
			lists:     []models.List{{UUID: "list-one", Attribs: models.JSON{"hosted_form": valid}}},
			requested: []string{"list-one"},
			want:      "List one",
		},
		{
			name:      "falls back when schema is missing",
			lists:     []models.List{{UUID: "list-one"}},
			requested: []string{"list-one"},
			want:      "Global",
		},
		{
			name:      "falls back when schema is invalid",
			lists:     []models.List{{UUID: "list-one", Attribs: models.JSON{"hosted_form": models.PublicSubscriptionForm{Fields: []models.PublicSubscriptionFormField{{Key: "bad key", Type: "text", Label: "Bad"}}}}}},
			requested: []string{"list-one"},
			want:      "Global",
		},
		{
			name: "uses first requested list",
			lists: []models.List{
				{UUID: "list-one", Attribs: models.JSON{"hosted_form": models.PublicSubscriptionForm{Heading: "First"}}},
				{UUID: "list-two", Attribs: models.JSON{"hosted_form": models.PublicSubscriptionForm{Heading: "Second"}}},
			},
			requested: []string{"list-two", "list-one"},
			want:      "Second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSubscriptionForm(global, tt.lists, tt.requested)
			if got.Heading != tt.want {
				t.Fatalf("heading = %q, want %q", got.Heading, tt.want)
			}
		})
	}
}

func TestCollectSubscriptionFormAttribs(t *testing.T) {
	fields := []models.PublicSubscriptionFormField{
		{Key: "company", Type: "text", Label: "Company", Required: true},
		{Key: "size", Type: "select", Label: "Team size", Options: []string{"1-10", "11-50"}},
		{Key: "updates", Type: "checkbox", Label: "Updates"},
		{Type: "consent", Label: "I agree", Required: true},
		{Key: "permission", Type: "consent", Label: "Store my consent"},
	}

	normalized, ok := normalizeSubscriptionForm(models.PublicSubscriptionForm{Fields: fields})
	if !ok {
		t.Fatal("expected configured fields to normalize")
	}

	attribs, err := collectSubscriptionFormAttribs(url.Values{
		"attribs.company":    {"WorkMate"},
		"attribs.size":       {"11-50"},
		"attribs.updates":    {"true"},
		"consent":            {"true"},
		"attribs.permission": {"true"},
	}, nil, normalized.Fields)
	if err != nil {
		t.Fatal(err)
	}
	if attribs["company"] != "WorkMate" || attribs["size"] != "11-50" || attribs["updates"] != true || attribs["permission"] != true {
		t.Fatalf("unexpected attributes: %#v", attribs)
	}
	if _, ok := attribs["consent"]; ok {
		t.Fatalf("keyless consent must not be persisted: %#v", attribs)
	}

	if _, err := collectSubscriptionFormAttribs(url.Values{"attribs.company": {"WorkMate"}, "attribs.size": {"100+"}, "attribs.consent": {"true"}}, nil, fields); err == nil {
		t.Fatal("invalid select option must fail")
	}
	if _, err := collectSubscriptionFormAttribs(url.Values{"attribs.company": {"WorkMate"}, "attribs.size": {"1-10"}}, nil, fields); err == nil {
		t.Fatal("unchecked required consent must fail")
	}
	if _, err := collectSubscriptionFormAttribs(url.Values{"attribs.company": {"WorkMate"}, "attribs.size": {"1-10"}, "attribs.consent": {"true"}, "attribs.unknown": {"x"}}, nil, fields); err == nil {
		t.Fatal("unknown field key must fail")
	}

	jsonAttribs, err := collectSubscriptionFormAttribs(nil, map[string]string{"": "true", "updates": "false"}, normalized.Fields)
	if err != nil || jsonAttribs["updates"] != false {
		t.Fatalf("expected keyless consent and unchecked checkbox to be accepted: %#v, %v", jsonAttribs, err)
	}
}

func TestMergeSubscriptionAttribsPreservesExistingAnswers(t *testing.T) {
	merged := mergeSubscriptionAttribs(models.JSON{"source": "import"}, models.JSON{"company": "WorkMate"})
	if merged["source"] != "import" || merged["company"] != "WorkMate" {
		t.Fatalf("subscriber attribs were not merged: %#v", merged)
	}
}
