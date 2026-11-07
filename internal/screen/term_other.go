//go:build !windows

package screen

import "os"

func enableVT(*os.File) (func(), error) { return func() {}, nil }
