package migrate

import (
	"path"
	"path/filepath"
	"runtime"
)

func Path(filename string) string {
	_, fp, _, ok := runtime.Caller(1)
	if !ok {
		panic("unable to get the current filename")
	}
	return path.Join(filepath.Dir(fp), filename)
}
