package main

import (
	"context"
	"flag"
	"os"

	"github.com/joho/godotenv"
	"github.com/nlopes/slack"
)

func main() {
	godotenv.Load()

	token := os.Getenv("API_TOKEN_FOR_SLACK")
	if len(token) == 0 {
		panic("require token on .env")
	}

	lang := flag.String("lang", "ja", "Language to speak")
	flag.Parse()

	ctx := context.Background()
	client := slack.New(token)
	NewSlackBot(client, *lang).Run(ctx)
}
