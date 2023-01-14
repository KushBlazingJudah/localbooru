package main

import (
	"log"
	"net/http"

	"github.com/KushBlazingJudah/localbooru"
)

func main() {
	lb := &localbooru.HTTP{BaseURL: "http://127.0.0.1:8081"}
	if err := lb.Open("lb.db"); err != nil {
		log.Fatal(err)
	}

	s := &http.Server{
		Addr:    ":8081",
		Handler: lb,
	}
	log.Fatal(s.ListenAndServe())
}
