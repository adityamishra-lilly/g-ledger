package main

import "github.com/adityamishra-lilly/g-ledger/internal/server"

func main() {
	server := &server.Server{
		Port: 7379,
	}
	server.StartServer()
}