package main

import (
	"net/http"
	"os"
)

func main() {
	InitState()
	go StartFetchers()
	go StartTelegramBot()
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/state", ApiStateHandler)
	http.HandleFunc("/ws", WsHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	http.ListenAndServe(":"+port, nil)
}
