package wsiwriter

import (
	"strings"
	"testing"
)

func TestValidateWSIImageType(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid label", "label", false},
		{"valid pyramid", "pyramid", false},
		{"valid associated", "associated", false},
		{"empty", "", true},
		{"unknown", "FOOBAR", true},
		{"uppercase rejected", "LABEL", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWSIImageType(c.input)
			if (err != nil) != c.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "wsi") {
				t.Errorf("error message should mention 'wsi': %v", err)
			}
		})
	}
}
