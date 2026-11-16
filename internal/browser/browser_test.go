package browser

import "testing"

func TestValidate(t *testing.T) {
	for _, ok := range []string{"https://github.com/o/r", "http://localhost:8080"} {
		if err := validate(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "--help", "file:///etc/passwd", "javascript:alert(1)", "-o/x"} {
		if err := validate(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
