package slug

import "testing"

func TestGenerate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"My Konos Monaco Royal", "my-konos-monaco-royal"},
		{"Mix Dunhill Blue x Scandal", "mix-dunhill-blue-x-scandal"},
		{"212 VIP Man", "212-vip-man"},
		{"  extra   spaces  ", "extra-spaces"},
		{"", "item"},
		{"***", "item"},
		{"Already-Slugged", "already-slugged"},
	}
	for _, c := range cases {
		if got := Generate(c.in); got != c.want {
			t.Errorf("Generate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
