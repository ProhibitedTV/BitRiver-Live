//go:build windows

package main

import "fmt"

func detectFreeDiskBytes(path string) (uint64, error) {
	return 0, fmt.Errorf("unsupported OS windows")
}
