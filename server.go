package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	portFlag = flag.Int("port", 8080, "Port to listen on")
)

// Define WebSocket upgrade settings
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Define global variables
var (
	clients     = make(map[*websocket.Conn]bool) // Map to store connected clients
	broadcast   = make(chan Message)             // Channel to broadcast messages to clients
	history     []string                         // Slice to store message history
	historyLock sync.Mutex                       // Mutex to synchronize access to message history
)

// Message represents a message sent from a client
type Message struct {
	Text string `json:"text"`
}

// handleWebSocket upgrades HTTP connection to WebSocket and handles communication
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	// Add client to clients map
	clients[conn] = true

	// Send history to client
	historyLock.Lock()
	for _, msg := range history {
		if err := conn.WriteJSON(Message{Text: msg}); err != nil {
			log.Println("Error writing history message:", err)
			break
		}
	}
	historyLock.Unlock()

	// Continuously read messages from the WebSocket connection
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Append message to history
		historyLock.Lock()
		history = append(history, msg.Text)
		historyLock.Unlock()

		// Broadcast message to all clients
		broadcast <- msg
	}
}

// broadcastMessages broadcasts messages to all connected clients
func broadcastMessages() {
	for {
		// Wait for a message from the broadcast channel
		msg := <-broadcast

		// Send message to all connected clients
		for client := range clients {
			if err := client.WriteJSON(msg); err != nil {
				log.Println("Error broadcasting message:", err)
				// Remove client if there's an error
				delete(clients, client)
				client.Close()
			}
		}
	}
}

func main() {
	flag.Parse()

	// Start broadcasting messages to clients
	go broadcastMessages()

	// Serve WebSocket connections
	http.HandleFunc("/ws", handleWebSocket)

	// Serve static files (e.g., HTML, CSS, JavaScript)
	http.Handle("/", http.FileServer(http.Dir("./public")))

	// Start HTTP server
	addr := fmt.Sprintf(":%d", *portFlag)
	fmt.Printf("Server listening on port %d\n", *portFlag)
	log.Fatal(http.ListenAndServe(addr, nil))
}
