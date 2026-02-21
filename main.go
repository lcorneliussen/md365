package main

import (
	"os"

	"github.com/larsc/md365/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
