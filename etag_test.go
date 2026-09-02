package scim

import "testing"

func TestVersionAndConditionalRequests(t *testing.T) {
	version, err := Version([]byte(`{"active":true,"id":"one"}`))
	if err != nil || len(version) != 66 || version[0] != '"' {
		t.Fatalf("Version() = (%q, %v)", version, err)
	}
	for _, test := range []struct {
		header string
		exists bool
		want   bool
	}{
		{"", true, true}, {version, true, true}, {"W/" + version, true, false}, {"*", true, true}, {"*", false, false}, {`"other"`, true, false},
	} {
		got, err := IfMatch(test.header, version, test.exists)
		if err != nil || got != test.want {
			t.Errorf("IfMatch(%q) = (%v, %v), want %v", test.header, got, err, test.want)
		}
	}
	for _, test := range []struct {
		header string
		exists bool
		want   bool
	}{
		{"", true, true}, {version, true, false}, {"W/" + version, true, false}, {"*", true, false}, {"*", false, true}, {`"other"`, true, true},
	} {
		got, err := IfNoneMatch(test.header, version, test.exists)
		if err != nil || got != test.want {
			t.Errorf("IfNoneMatch(%q) = (%v, %v), want %v", test.header, got, err, test.want)
		}
	}
	for _, invalid := range []string{"bad", `"unterminated`, `"bad space"`, `"one" garbage`} {
		if _, err := IfMatch(invalid, version, true); err == nil {
			t.Errorf("invalid tag %q passed", invalid)
		}
	}
	if got, err := Version(nil); err == nil || got != "" {
		t.Fatalf("Version(nil) = (%q, %v)", got, err)
	}
}
