package server

import (
	"net"
	"testing"

	"github.com/adityamishra-lilly/g-ledger/config"
)

func TestServerListen(t *testing.T) {
	server := &Server{
		Port: config.Port,
		Host: config.Host,
	}
	
	listener, err := server.listen()
	if err != nil {
		t.Fatalf("Expected server to listen to port %d, error: %v", config.Port, err)
	}

	defer listener.Close()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal("Error connecting to listener")
	}
	defer conn.Close()
}