package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket upgrader to convert HTTP connections to WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// Global variables to manage connected clients and broadcasting messages
var (
	clients   = make(map[*websocket.Conn]bool) // Tracks active WebSocket connections
	broadcast = make(chan struct {
		sender  *websocket.Conn
		message []byte
	})
	mutex sync.Mutex // Mutex for safe access to shared data
)

// WebSocket handler: Manages individual client connections
func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer func() {
		mutex.Lock()
		delete(clients, conn) // Remove client on disconnect
		mutex.Unlock()
		conn.Close()
		log.Println("Client disconnected")
	}()

	// Register the new client
	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()
	log.Println("New client connected")

	// Send a welcome message
	if err := conn.WriteMessage(websocket.TextMessage, []byte("Welcome to the WebSocket server!")); err != nil {
		log.Println("Error sending welcome message:", err)
		return
	}

	// Continuously read messages from the client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}
		log.Println("Received message:", string(message))
		broadcast <- struct {
			sender  *websocket.Conn
			message []byte
		}{sender: conn, message: message} // Send sender info with the message
	}
}

// Broadcasts messages to all connected clients
func handleBroadcast() {
	for {
		// Receive message from the broadcast channel
		data := <-broadcast
		sender := data.sender
		message := data.message

		mutex.Lock()
		for client := range clients {
			if client != sender { // Skip sending back to the sender
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Println("Error broadcasting message:", err)
					client.Close()
					delete(clients, client) // Remove disconnected clients
				}
			}
		}
		mutex.Unlock()
	}
}

// Sends periodic ping messages to maintain WebSocket connections
func keepAlive() {
	ticker := time.NewTicker(30 * time.Second) // Pings every 30 seconds
	defer ticker.Stop()

	for {
		<-ticker.C
		mutex.Lock()
		for client := range clients {
			if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Ping error:", err)
				client.Close()
				delete(clients, client) // Remove unresponsive clients
			}
		}
		mutex.Unlock()
	}
}

// Simple HTTP handler for the home page
func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome to the WebSocket Server!")
}

// Set up HTTP routes
func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsHandler)
}

func main() {
	fmt.Println("WebSocket server running on :8080")

	// Start the broadcaster and keep-alive mechanisms
	go handleBroadcast()
	go keepAlive()

	// Set up routes and start the HTTP server
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
