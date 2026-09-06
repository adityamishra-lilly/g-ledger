package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adityamishra-lilly/g-ledger/config"
	"github.com/adityamishra-lilly/g-ledger/internal/server"
)

func main() {	
	if err := run(); err != nil {
		log.Printf("Server stopped with error: %v", err)
		os.Exit(1)
	}

}


func run() error{
		// Load configurations
	config.Parse()

	// Start the server
	server := server.Server{
		Port: config.Port,
		Host: config.Host,
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.StartServer()
	}()

	select {
	case err := <-errCh:
		return err

	case <- signalCtx.Done():
		log.Println("Shutdown signal received")
		stop()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		serverErr := <-errCh

		if serverErr != nil{
			return serverErr
		}
	
		return shutdownErr
	}
}