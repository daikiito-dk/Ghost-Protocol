package main

import (
	"log"
	"net/http"

	"ghost-protocol/internal/room"
	ws "ghost-protocol/internal/websocket"
)

func main() {
	manager := room.NewManager()
	hub := ws.NewHub(manager)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("web")))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ws", hub.Handle)

	server := &http.Server{Addr: ":8080", Handler: mux}
	log.Println("Ghost Protocol listening on http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}
