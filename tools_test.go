package toolkit

import "testing"

func TestGenerateRandomString(t *testing.T) {
	var tools Tools

	s := tools.GenerateRandomString(10)
	if len(s) != 10 {
		t.Errorf("Length of generated string is not 10: %d", len(s))
	}
}
