package argon

import "testing"

func TestRegistry(t *testing.T) {
	r := newRegistry()
	for _, n := range []string{"b", "a", "c.d-e_f"} {
		if err := r.Add(&toolDef{Name: n, Handler: textHandler(n)}); err != nil {
			t.Fatalf("add %q: %v", n, err)
		}
	}
	if err := r.Add(&toolDef{Name: "a", Handler: textHandler("")}); err == nil {
		t.Error("duplicate must fail")
	}
	for _, bad := range []string{"", "has space", "slash/no"} {
		if err := r.Add(&toolDef{Name: bad, Handler: textHandler("")}); err == nil {
			t.Errorf("name %q must be rejected", bad)
		}
	}
	if err := r.Add(&toolDef{Name: "nohandler"}); err == nil {
		t.Error("missing handler must fail")
	}
	var names []string
	for _, tool := range r.All() {
		names = append(names, tool.Name)
	}
	if got := len(names); got != 3 || names[0] != "b" || names[1] != "a" {
		t.Fatalf("order = %v, want registration order", names)
	}
}
