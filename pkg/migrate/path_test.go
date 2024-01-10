package migrate

import (
	"strings"
	"testing"
)

func TestCanonicalPath(t *testing.T) {
	d := CanonicalPath("")

	if !strings.HasPrefix(d, "/") {
		t.Fatalf("invalid prefix: %s", d)
	}

	if !strings.HasSuffix(d, "/pkg/migrate") {
		t.Fatalf("invalid suffix: %s", d)
	}
}
