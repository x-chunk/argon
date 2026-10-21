package argon

import "testing"

func TestPack(t *testing.T) {
	type obj struct {
		N int `json:"n"`
	}
	tests := []struct {
		name       string
		out        any
		structured bool
		wantText   string
		wantStruct bool
	}{
		{"string", "hi", false, "hi", false},
		{"struct", obj{N: 1}, true, `{"n":1}`, true},
		{"slice is text only", []string{"a"}, false, `["a"]`, false},
		{"nil slice", []string(nil), false, `[]`, false},
		{"nil map", map[string]int(nil), false, `{}`, false},
		{"number", 42, false, `42`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := pack(tt.out, tt.structured)
			if err != nil {
				t.Fatal(err)
			}
			if got := res.Content[0].(TextContent).Text; got != tt.wantText {
				t.Errorf("text = %q, want %q", got, tt.wantText)
			}
			if (res.Structured != nil) != tt.wantStruct {
				t.Errorf("structured = %v, want present=%v", res.Structured, tt.wantStruct)
			}
		})
	}
}
