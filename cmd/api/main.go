package main

import (
	"log"

	"github.com/pelyams/simpler_go_service/cmd/api/app"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
