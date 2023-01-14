package main

import (
	"log"
	"net/http"

	"github.com/KushBlazingJudah/localbooru"
)

func main() {
	lb := &localbooru.HTTP{}
	if err := lb.Open("lb.db"); err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(":8081", lb))
}
