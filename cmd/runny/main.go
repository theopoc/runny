package main

import (
	"os"

	"github.com/theopoc/runny/internal/app"
)

func main() {
	os.Exit(app.Run(app.Options{Args: os.Args[1:]}))
}
