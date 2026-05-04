package main

import (
	"os"

	"github.com/ffreis/platform-runner/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
