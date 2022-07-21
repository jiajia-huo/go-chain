package main

import (
	"fmt"
)

var (
	version = "v0.1.0"
)

func Version() string {
	return fmt.Sprintf("%s built from git", version)
}
