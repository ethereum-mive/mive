package main

import (
	"fmt"
	"os"

	"github.com/ethereum-mive/mive/internal/flags"
)

const (
	clientIdentifier = "mive" // Client identifier
)

var app = flags.NewApp("the mive command line interface")

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
