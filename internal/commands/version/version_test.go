package version

import "testing"

func TestCommitFromPseudoVersion(t *testing.T) {
	cases := map[string]string{
		"v0.0.0-20261023144319-c140a493734d":       "c140a493734d",
		"v0.0.0-20261023144319-c140a493734d+dirty": "",
		"v1.2.3": "",
		"dev":    "",
	}
	for in, want := range cases {
		if got := commitFromPseudoVersion(in); got != want {
			t.Errorf("commitFromPseudoVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
