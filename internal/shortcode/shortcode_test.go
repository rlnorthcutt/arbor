package shortcode

import (
	"testing"
)

func TestTokenFor(t *testing.T) {
	if tokenFor(0) != "<!--ARBOR-SC-0-->" {
		t.Errorf("expected '<!--ARBOR-SC-0-->', got '%s'", tokenFor(0))
	}
	if tokenFor(5) != "<!--ARBOR-SC-5-->" {
		t.Errorf("expected '<!--ARBOR-SC-5-->', got '%s'", tokenFor(5))
	}
}

func TestParseParams(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{
			input:    `key="value"`,
			expected: map[string]string{"key": "value"},
		},
		{
			input:    `foo=bar baz="qux quux"`,
			expected: map[string]string{"foo": "bar", "baz": "qux quux"},
		},
		{
			input:    "",
			expected: map[string]string{},
		},
		{
			input:    `type="warning" title="Watch Out"`,
			expected: map[string]string{"type": "warning", "title": "Watch Out"},
		},
	}

	for _, tc := range tests {
		result := parseParams(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("parseParams(%q): expected %d params, got %d", tc.input, len(tc.expected), len(result))
			continue
		}
		for k, v := range tc.expected {
			if result[k] != v {
				t.Errorf("parseParams(%q): expected %s=%q, got %q", tc.input, k, v, result[k])
			}
		}
	}
}

func TestPreProcess_SelfClosing(t *testing.T) {
	p := New()
	input := `# Hello

{{% partial "displays/card" key="value" %}}

Some text after.`

	result := p.PreProcess(input)

	if result == input {
		t.Error("PreProcess should have modified input")
	}
	if len(p.extracted) != 1 {
		t.Fatalf("expected 1 extracted shortcode, got %d", len(p.extracted))
	}
	if p.extracted[0].partial != "displays/card" {
		t.Errorf("expected partial 'displays/card', got '%s'", p.extracted[0].partial)
	}
	if p.extracted[0].params["key"] != "value" {
		t.Errorf("expected param key='value', got '%s'", p.extracted[0].params["key"])
	}
	if p.extracted[0].body != "" {
		t.Error("expected empty body for self-closing shortcode")
	}
	// Token should be in result
	if !containsToken(result, 0) {
		t.Error("expected token __ARBOR_SC_0__ in result")
	}
}

func TestPreProcess_BlockShortcode(t *testing.T) {
	p := New()
	input := `Before

{{% partial "partials/callout" type="warning" %}}Watch out for this!{{% /partial %}}

After`

	result := p.PreProcess(input)

	if len(p.extracted) != 1 {
		t.Fatalf("expected 1 extracted shortcode, got %d", len(p.extracted))
	}
	if p.extracted[0].partial != "partials/callout" {
		t.Errorf("expected partial 'partials/callout', got '%s'", p.extracted[0].partial)
	}
	if p.extracted[0].params["type"] != "warning" {
		t.Errorf("expected param type='warning', got '%s'", p.extracted[0].params["type"])
	}
	if p.extracted[0].body != "Watch out for this!" {
		t.Errorf("expected body 'Watch out for this!', got '%s'", p.extracted[0].body)
	}
	if !containsToken(result, 0) {
		t.Error("expected token __ARBOR_SC_0__ in result")
	}
}

func TestPreProcess_MultipleShortcodes(t *testing.T) {
	p := New()
	input := `{{% partial "displays/card" id="1" %}}

Some text

{{% partial "displays/teaser" id="2" %}}`

	result := p.PreProcess(input)

	if len(p.extracted) != 2 {
		t.Fatalf("expected 2 extracted shortcodes, got %d", len(p.extracted))
	}
	if !containsToken(result, 0) {
		t.Error("expected token __ARBOR_SC_0__")
	}
	if !containsToken(result, 1) {
		t.Error("expected token __ARBOR_SC_1__")
	}
}

func TestPreProcess_NoShortcodes(t *testing.T) {
	p := New()
	input := "# Just markdown\n\nNo shortcodes here."
	result := p.PreProcess(input)

	if result != input {
		t.Error("PreProcess should not modify input with no shortcodes")
	}
	if len(p.extracted) != 0 {
		t.Errorf("expected 0 extracted shortcodes, got %d", len(p.extracted))
	}
}

func TestPostProcess_NoTokens(t *testing.T) {
	p := New()
	_ = p.PreProcess("no shortcodes")

	html := "<p>Hello world</p>"
	result := p.PostProcess(html, "/templates", nil)

	if result != html {
		t.Errorf("PostProcess should not modify HTML with no tokens: got %q", result)
	}
}

func containsToken(s string, i int) bool {
	token := tokenFor(i)
	return len(s) > 0 && (s == token || len(s) > len(token) && containsSubstring(s, token))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
