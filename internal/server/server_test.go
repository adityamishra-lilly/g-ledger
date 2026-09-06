package server

// import (
// 	"net"
// 	"testing"

// 	"github.com/adityamishra-lilly/g-ledger/config"
// )

// func TestServerListen(t *testing.T) {
// 	server.

// 	listener, err := server.Listen()
// 	if err != nil {
// 		t.Fatalf("Expected server to listen to port %d, error: %v", config.Port, err)
// 	}

// 	defer listener.Close()

// 	conn, err := net.Dial("tcp", listener.Addr().String())
// 	if err != nil {
// 		t.Fatal("Error connecting to listener")
// 	}
// 	defer conn.Close()
// }