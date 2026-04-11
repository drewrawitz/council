package main

import (
	"os"

	"council/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
