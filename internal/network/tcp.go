package network

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/gauravnetes/go-cask-db/internal/engine"
)

// Server represents our TCP database server
type Server struct {
	db      *engine.DB
	address string
}

// NewServer creates a new network server attached to our engine
func NewServer(db *engine.DB, address string) *Server {
	return &Server{
		db:      db,
		address: address,
	}
}

// Start opens the TCP port and begins accepting client connections
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("go-cask-db is alive and listening on %s\n", s.address)
	fmt.Println("Waiting for clients to connect...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		// Spin up a new Goroutine for EVERY client that connects!
		// This means 100 people can query the database at the exact same time.
		go s.handleConnection(conn)
	}
}

// handleConnection reads raw text commands from a specific client
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	// A nice welcome message for the client
	conn.Write([]byte("Connected to go-cask-db. Type a command (e.g., SET key value, GET key):\n"))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// Parse the command
		parts := strings.SplitN(text, " ", 3)
		if len(parts) == 0 {
			continue
		}
		command := strings.ToUpper(parts[0])

		switch command {
		case "PING":
			conn.Write([]byte("PONG\n"))

		case "SET":
			if len(parts) < 3 {
				conn.Write([]byte("ERROR: Usage: SET <key> <value>\n"))
				continue
			}
			key, value := parts[1], parts[2]
			err := s.db.Put(key, []byte(value))
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
			} else {
				conn.Write([]byte("OK\n"))
			}

		case "GET":
			if len(parts) < 2 {
				conn.Write([]byte("ERROR: Usage: GET <key>\n"))
				continue
			}
			key := parts[1]
			val, err := s.db.Get(key)
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
			} else {
				// We append a newline so it formats nicely in the terminal
				conn.Write([]byte(fmt.Sprintf("%s\n", string(val))))
			}

		case "COMPACT":
			err := s.db.Compact()
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
			} else {
				conn.Write([]byte("OK: Compaction complete\n"))
			}

		case "EXIT", "QUIT":
			conn.Write([]byte("Goodbye!\n"))
			return

		default:
			conn.Write([]byte(fmt.Sprintf("ERROR: Unknown command '%s'\n", command)))
		}
	}
}