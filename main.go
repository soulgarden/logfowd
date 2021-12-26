package main

import (
	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/cmd"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cmd.Execute()
}
