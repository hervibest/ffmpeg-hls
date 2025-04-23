package util

import (
	"os"
)

func DeleteDir(path string) error {
	return os.RemoveAll(path)
}
