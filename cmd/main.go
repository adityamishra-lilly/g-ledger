package main

import (
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
	server.StartServer()
}