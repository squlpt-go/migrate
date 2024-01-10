package migrate

import (
	"errors"
	"path/filepath"
	"runtime"
)

// these funcs stolen from: https://stackoverflow.com/a/70491592/2626654

func CurrentDirname() (string, error) {
	filename, err := currentFilename()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filename), nil
}

func currentFilename() (string, error) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("unable to get the current filename")
	}
	return filename, nil
}
