package main

import (
	_ "github.com/mailru/easyjson/gen"
	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/cmd"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cmd.Execute()
}
