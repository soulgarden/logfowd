package main

import (
	_ "github.com/mailru/easyjson/gen"
	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/cmd"
	_ "go.uber.org/automaxprocs"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cmd.Execute()
}
