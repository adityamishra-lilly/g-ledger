package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)


type Server struct {
	Port    int
	Host    string
	listener net.Listener  
	clients int32
	clientWg sync.WaitGroup
	connMu sync.Mutex
	conns map[net.Conn]struct{}
	draining atomic.Bool
}

func (server *Server) listen() (net.Listener, error) {
	log.Printf("Starting the tcp server at port %d", server.Port)
	listener, err := net.Listen("tcp",server.Host+":"+ strconv.Itoa(server.Port))

	if err != nil{
		return nil, err
	}

	server.listener = listener

	return server.listener,nil
}

func (server *Server) registerConnection(conn net.Conn) {
	server.connMu.Lock()
	defer server.connMu.Unlock()

	server.conns[conn] = struct{}{}
}


func (server *Server) unregisterConnection(conn net.Conn) {
	server.connMu.Lock()
	defer server.connMu.Unlock()

	delete(server.conns, conn)
}


func (server *Server) StartServer() error{
	server.conns = make(map[net.Conn]struct{})
	listener, err := server.listen()
	if err != nil{
	 log.Printf("Error starting server: %v", err)
	 return err
	}

	defer listener.Close()
	for{
		conn, err := listener.Accept()
	  if err != nil {
			if server.draining.Load() {
				log.Println("Server stopped accepting new connections")
				return nil
			}
			log.Printf("Error accepting client: %v", err)
			return err
		}
		server.registerConnection(conn)
		server.clientWg.Add(1)
		atomic.AddInt32(&server.clients, 1)

		log.Println("new client connected with address", conn.RemoteAddr())

		go func() {
			defer server.clientWg.Done()
			server.processClients(conn)
		}()

	}

}

func readCommand(conn net.Conn) (string, error){
	conn.SetReadDeadline(time.Now().Add(30* time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil{
		return "", err
	}
	return string(buf[:n]), nil
}

func echoCommand(conn net.Conn, cmd string) error{
	if _, err := conn.Write([]byte(cmd)); err != nil{
		return err
	}
	return nil
}

func (server *Server) processClients(conn net.Conn) {
	defer server.closeConnection(conn)
	for{
		cmd, err := readCommand(conn)
		if err != nil{
			if server.draining.Load() {
				log.Printf("connection closed during shutdowm: %v", conn.RemoteAddr())
			} else {
				log.Printf("Error reading command: %v", err)
			}
			
			return
		}
		err = echoCommand(conn, cmd)
		if err != nil {
			log.Printf("Error writing command: %v", err)
			return
		}
	}

}


func (server *Server) closeConnection(conn net.Conn) {
	server.unregisterConnection(conn)
	conn.Close()	
	atomic.AddInt32(&server.clients, -1)
	log.Println("client disconnected", conn.RemoteAddr(), "concurrent clients", server.clients)
}

func (server *Server) setConnDeadline() {
	server.connMu.Lock()
	defer server.connMu.Unlock()

	for conn := range server.conns {
		conn.SetReadDeadline(time.Now())
	}
}

func (server *Server) forceCloseConnections() int{
	server.connMu.Lock()
	defer server.connMu.Unlock()
	count := 0

	for conn := range server.conns {
		conn.Close()
		count++
	}
	return count
}


func (server *Server) Shutdown(ctx context.Context) error {
	server.draining.Store(true)
	server.listener.Close()
	server.setConnDeadline()
	done := make(chan struct{})
	go func() {
		server.clientWg.Wait()
		close(done)
	}()
	select {
	case <- done:
		return nil

	case <- ctx.Done():
		count := server.forceCloseConnections()
		log.Printf("Shutdown timeout: forcibly closed %d connections", count)
		return fmt.Errorf("Shutdown error")
	}


	
}


