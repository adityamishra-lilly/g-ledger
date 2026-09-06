package server

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

type State uint8

const (
	StateWaiting State = iota
	StateBusy
	StateShutdown
)

type Server struct {
	Port    int
	Host    string
	clients int32
	state atomic.Uint32
	listener net.Listener  
}

func (server *Server) getState() State{
	return State(server.state.Load())
}
func (server *Server) setState(state State) {
	server.state.Store(uint32(state))
}




func (server *Server) listen() (net.Listener, error) {
	log.Printf("Starting the tcp server at port %d", server.Port)
	listener, err := net.Listen("tcp",server.Host+":"+ strconv.Itoa(server.Port))
	server.listener = listener
	if err != nil{
		return nil, err
	}

	return server.listener,nil
}


func (server *Server) StartServer(wg *sync.WaitGroup) error{
	defer wg.Done()
	server.setState(StateWaiting)
	listener, err := server.listen()
	if err != nil{
	 log.Printf("Error starting server: %v", err)
	 return err
	}

	defer listener.Close()
	for{
		conn, err := listener.Accept()
	  if err != nil{
			log.Printf("Error accepting client: %v", err)
			return err
		}
		server.setState(StateBusy)
		atomic.AddInt32(&server.clients, 1)

		log.Println("new client connected with address", conn.RemoteAddr())

		go server.processClients(conn)
		
	}

}

func readCommand(conn net.Conn) (string, error){
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
			log.Printf("Error reading command: %v", err)
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
	conn.Close()
	clients := atomic.AddInt32(&server.clients, -1)
	if clients == 0 {
		server.setState(StateWaiting)
	}
	log.Println("client disconnected", conn.RemoteAddr(), "concurrent clients", server.clients)
}


func (server *Server) Shutdown(sigs chan os.Signal, wg *sync.WaitGroup) {
	defer wg.Done()

	// Blocks further execution until signal received
	<-sigs

	for server.getState() == StateBusy{
		
	}

	if server.getState() == StateWaiting{
		server.listener.Close()
		server.setState(StateShutdown)
	}




}