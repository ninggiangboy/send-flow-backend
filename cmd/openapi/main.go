package main

import (
	"log"
	"os"

	"github.com/ninggiangboy/send-flow/backend/internal/apps/api"
)

func main() {
	spec, err := api.GenerateOpenAPIYAML()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(spec); err != nil {
		log.Fatal(err)
	}
}
