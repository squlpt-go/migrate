package migrate

import (
	"strings"
	"testing"
)

func TestCurrentDirname(t *testing.T) {
	d, err := CurrentDirname()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(d, "/") {
		t.Fatalf("invalid prefix: %s", d)
	}

	if !strings.HasSuffix(d, "/pkg/migrate") {
		t.Fatalf("invalid suffix: %s", d)
	}
}
