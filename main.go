package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"Pobeda/applayer"
	"Pobeda/com"
	"Pobeda/datalayer"
)

func main() {
	// test com connection
	// log.Println(com.Connect(&com.Config{
	// 	Name:     "/dev/ttyS0",
	// 	BaudRate: 115200,
	// }))

	com.Init()
	defer com.Close()
	datalayer.Init()
	defer datalayer.Close()
	applayer.Init()

	// init application layer and start listen to it
	srv := http.Server{
		Addr: ":8080",
	}
	http.HandleFunc("/chat", applayer.Connect)

	idleConnsClosed := make(chan struct{})
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs

		log.Printf("got %s, shutting down server", sig)
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP server Shutdown: %s", err)
		}
		close(idleConnsClosed)
	}()

	log.Println("Starting HTTP server on :8080...")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("HTTP server ListenAndServe: %s", err)
	}

	<-idleConnsClosed
}
