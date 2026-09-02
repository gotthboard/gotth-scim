package scim

import "testing"

func TestParseEqualityFilter(t *testing.T) {
	got, err := ParseEqualityFilter(`UserName eq "member@example.com"`, "userName")
	if err != nil || got != "member@example.com" {
		t.Fatalf("ParseEqualityFilter() = (%q, %v)", got, err)
	}
	for _, raw := range []string{"", `displayName eq "Member"`, `userName co "member"`, `userName eq member`, "userName eq \"bad\nvalue\""} {
		if _, err := ParseEqualityFilter(raw, "userName"); err == nil {
			t.Errorf("filter %q passed", raw)
		}
	}
}
