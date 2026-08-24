package campaign

import "testing"

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		vars map[string]string
		want string
	}{
		{
			name: "value present, comma before",
			tmpl: "Здравствуйте, {{name}}! Скидка 20%",
			vars: map[string]string{"name": "Aigul"},
			want: "Здравствуйте, Aigul! Скидка 20%",
		},
		{
			name: "value empty, comma before dropped",
			tmpl: "Здравствуйте, {{name}}! Скидка 20%",
			vars: map[string]string{"name": ""},
			want: "Здравствуйте! Скидка 20%",
		},
		{
			name: "value missing entirely, comma before dropped",
			tmpl: "Здравствуйте, {{name}}! Скидка 20%",
			vars: map[string]string{},
			want: "Здравствуйте! Скидка 20%",
		},
		{
			name: "value empty, comma after dropped (no separator before)",
			tmpl: "Hi {{name}}, welcome",
			vars: map[string]string{"name": ""},
			want: "Hi welcome",
		},
		{
			name: "value empty, semicolon before dropped",
			tmpl: "Order #123; {{note}} thanks",
			vars: map[string]string{"note": ""},
			want: "Order #123 thanks",
		},
		{
			name: "value empty, em-dash before dropped",
			tmpl: "Hello — {{name}}, how are you",
			vars: map[string]string{"name": ""},
			want: "Hello, how are you",
		},
		{
			name: "value empty, no adjacent separator either side",
			tmpl: "Hello[{{name}}]world",
			vars: map[string]string{"name": ""},
			want: "Hello[]world",
		},
		{
			name: "value whitespace-only counts as empty",
			tmpl: "Hi, {{name}}!",
			vars: map[string]string{"name": "   "},
			want: "Hi!",
		},
		{
			name: "value is trimmed",
			tmpl: "Hi {{name}}!",
			vars: map[string]string{"name": "  Aigul  "},
			want: "Hi Aigul!",
		},
		{
			name: "multiple variables, mixed present/empty",
			tmpl: "Здравствуйте, {{name}}! Ваш заказ {{order}} готов.",
			vars: map[string]string{"name": "Bota", "order": ""},
			want: "Здравствуйте, Bota! Ваш заказ готов.",
		},
		{
			name: "multiple variables, all empty",
			tmpl: "Здравствуйте, {{name}}! Ваш заказ {{order}} готов.",
			vars: map[string]string{},
			want: "Здравствуйте! Ваш заказ готов.",
		},
		{
			name: "no variables at all",
			tmpl: "Скидка 20% на всё!",
			vars: map[string]string{},
			want: "Скидка 20% на всё!",
		},
		{
			name: "unknown token left as literal text (no closing braces matched)",
			tmpl: "Hi {name}!",
			vars: map[string]string{"name": "Aigul"},
			want: "Hi {name}!",
		},
		{
			name: "token with internal whitespace still matches",
			tmpl: "Hi {{ name }}!",
			vars: map[string]string{"name": "Aigul"},
			want: "Hi Aigul!",
		},
		{
			name: "leading/trailing whitespace in template trimmed",
			tmpl: "  Hi {{name}}!  ",
			vars: map[string]string{"name": "Aigul"},
			want: "Hi Aigul!",
		},
		{
			name: "collapses a double space produced by two adjacent drops",
			tmpl: "Hi {{title}} {{name}}!",
			vars: map[string]string{"title": "", "name": ""},
			want: "Hi !",
		},
		{
			name: "empty template",
			tmpl: "",
			vars: map[string]string{"name": "Aigul"},
			want: "",
		},
		{
			name: "variable at the very start of the template",
			tmpl: "{{name}}, здравствуйте!",
			vars: map[string]string{"name": ""},
			want: "здравствуйте!",
		},
		{
			name: "variable at the very end of the template",
			tmpl: "Здравствуйте, {{name}}",
			vars: map[string]string{"name": ""},
			want: "Здравствуйте",
		},
		{
			name: "variable is the whole template",
			tmpl: "{{name}}",
			vars: map[string]string{"name": ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(tt.tmpl, tt.vars)
			if got != tt.want {
				t.Errorf("Render(%q, %v) = %q, want %q", tt.tmpl, tt.vars, got, tt.want)
			}
		})
	}
}

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want []string
	}{
		{"no variables", "Скидка 20%!", nil},
		{"one variable", "Hi {{name}}!", []string{"name"}},
		{"multiple in order", "{{name}}, order {{order}} is ready at {{name}}'s address", []string{"name", "order"}},
		{"dedup repeats", "{{a}} {{b}} {{a}}", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVariables(tt.tmpl)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractVariables(%q) = %v, want %v", tt.tmpl, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractVariables(%q)[%d] = %q, want %q", tt.tmpl, i, got[i], tt.want[i])
				}
			}
		})
	}
}
