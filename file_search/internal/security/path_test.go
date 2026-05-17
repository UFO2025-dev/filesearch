package security

import (
	"testing"
)

func TestValidatePath(t *testing.T) {
	root := "/home/user/docs"

	tests := []struct {
		target  string
		wantErr bool
	}{
		{"/home/user/docs/file.txt", false},
		{"/home/user/docs/sub/file.pdf", false},
		{"/home/user/docs", false},
		{"/home/user/docs/../other/secret", true},
		{"/home/user/secret", true},
		{"/etc/passwd", true},
		{"/home/user/docs/../../etc/passwd", true},
	}

	for _, tt := range tests {
		err := ValidatePath(root, tt.target)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePath(%q) error=%v, wantErr=%v", tt.target, err, tt.wantErr)
		}
	}
}
