package main

import (
	"os"

	"github.com/lcorneliussen/md365/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
