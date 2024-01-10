package migrate

import (
	"path/filepath"
	"runtime"
)

// these funcs stolen from: https://stackoverflow.com/a/70491592/2626654

func CurrentDirname() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("unable to get the current filename")
	}
	return filepath.Dir(filename)
}
