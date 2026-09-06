package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/adityamishra-lilly/g-ledger/config"
	"github.com/adityamishra-lilly/g-ledger/internal/server"
)

func main() {
	// Load configurations
	config.Parse()

	// Start the server
	server := &server.Server{
		Port: config.Port,
		Host: config.Host,
	}

	var wg sync.WaitGroup
	var sigs chan os.Signal = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM,syscall.SIGINT)
	wg.Add(2)

	go server.Shutdown(sigs, &wg)
	go server.StartServer(&wg)

	wg.Wait()
}