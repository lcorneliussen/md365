package main

import (
	"os"

	"github.com/lcorneliussen/md365/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
