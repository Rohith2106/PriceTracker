package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

type PriceCheckRequest struct {
	URL         string  `json:"url"`
	TargetPrice float64 `json:"targetPrice"`
}

type PriceCheckResponse struct {
	CurrentPrice  float64 `json:"currentPrice"`
	TargetPrice   float64 `json:"targetPrice"`
	IsBelowTarget bool    `json:"isBelowTarget"`
	PriceString   string  `json:"priceString"`
	Success       bool    `json:"success"`
	Message       string  `json:"message"`
}

type TrackingRequest struct {
	URL         string  `json:"url"`
	TargetPrice float64 `json:"targetPrice"`
	ID          string  `json:"id"`
}

type PriceAlert struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	CurrentPrice float64 `json:"currentPrice"`
	TargetPrice  float64 `json:"targetPrice"`
	PriceString  string  `json:"priceString"`
	Timestamp    string  `json:"timestamp"`
}

type Client struct {
	conn *websocket.Conn
	send chan PriceAlert
}

var (
	clients       = make(map[*Client]bool)
	trackingItems = make(map[string]TrackingRequest)
	mu            sync.RWMutex
	upgrader      = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/api/check-price", checkPriceHandler).Methods("POST")
	r.HandleFunc("/api/track-price", trackPriceHandler).Methods("POST")
	r.HandleFunc("/api/untrack-price", untrackPriceHandler).Methods("POST")
	r.HandleFunc("/api/tracked-items", getTrackedItemsHandler).Methods("GET")
	r.HandleFunc("/ws", handleWebSocket)
	r.HandleFunc("/api/health", healthHandler).Methods("GET")

	// Start price monitoring goroutine
	go monitorPrices()

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(r)

	fmt.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func checkPriceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req PriceCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" || req.TargetPrice <= 0 {
		response := PriceCheckResponse{
			Success: false,
			Message: "Invalid URL or target price",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	priceString, currentPrice, err := scrapePrice(req.URL)
	if err != nil {
		response := PriceCheckResponse{
			Success: false,
			Message: fmt.Sprintf("Unable to fetch price: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	isBelowTarget := currentPrice <= req.TargetPrice

	response := PriceCheckResponse{
		CurrentPrice:  currentPrice,
		TargetPrice:   req.TargetPrice,
		IsBelowTarget: isBelowTarget,
		PriceString:   priceString,
		Success:       true,
		Message:       "Price check successful",
	}

	// If price is below target, send notification immediately
	if isBelowTarget {
		// Generate a temporary ID for this check
		tempID := fmt.Sprintf("check-%d", time.Now().Unix())

		// Send notification without adding to tracking
		go func() {
			alert := PriceAlert{
				ID:           tempID,
				URL:          req.URL,
				CurrentPrice: currentPrice,
				TargetPrice:  req.TargetPrice,
				PriceString:  priceString,
				Timestamp:    time.Now().Format(time.RFC3339),
			}

			// Send to all connected WebSocket clients
			mu.RLock()
			clientCount := len(clients)
			log.Printf("Sending immediate alert to %d connected clients", clientCount)
			for client := range clients {
				select {
				case client.send <- alert:
					log.Printf("Immediate alert sent to client successfully")
				default:
					log.Printf("Client channel full, closing connection")
					close(client.send)
					delete(clients, client)
				}
			}
			mu.RUnlock()

			log.Printf("Immediate price alert sent for %s: ₹%s (target: ₹%.2f)", req.URL, priceString, req.TargetPrice)
		}()
	}

	json.NewEncoder(w).Encode(response)
}

func scrapePrice(url string) (string, float64, error) {
	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
	)

	// Add multiple domains to avoid blocking
	c.AllowedDomains = []string{"www.amazon.in", "amazon.in"}

	var priceString string

	// Multiple selectors to try
	c.OnHTML(".a-price-whole, .a-price-range .a-offscreen, .a-price .a-offscreen, .a-price-symbol + .a-price-whole", func(e *colly.HTMLElement) {
		if priceString == "" {
			priceString = strings.TrimSpace(e.Text)
		}
	})

	// Set realistic headers to avoid detection
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error occurred: %v", err)
	})

	// Add delay to avoid rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*amazon.*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	err := c.Visit(url)
	if err != nil {
		return "", 0, err
	}

	if priceString == "" {
		return "", 0, fmt.Errorf("price not found")
	}

	// Parse Indian price format (e.g., "60,100" to 60100)
	cleanPrice := strings.ReplaceAll(priceString, ",", "")
	cleanPrice = strings.ReplaceAll(cleanPrice, "₹", "")
	cleanPrice = strings.TrimSpace(cleanPrice)

	price, err := strconv.ParseFloat(cleanPrice, 64)
	if err != nil {
		return priceString, 0, fmt.Errorf("failed to parse price: %v", err)
	}

	return priceString, price, nil
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	log.Printf("WebSocket connection established successfully")
	client := &Client{
		conn: conn,
		send: make(chan PriceAlert, 256),
	}

	mu.Lock()
	clients[client] = true
	log.Printf("Total WebSocket clients connected: %d", len(clients))
	mu.Unlock()

	go client.writePump()
	go client.readPump()
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
		mu.Lock()
		delete(clients, c)
		mu.Unlock()
	}()

	for {
		select {
		case alert, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(alert); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer c.conn.Close()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// Track price handler
func trackPriceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req TrackingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" || req.TargetPrice <= 0 || req.ID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid URL, target price, or ID",
		})
		return
	}

	mu.Lock()
	trackingItems[req.ID] = req
	mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Price tracking started",
		"id":      req.ID,
	})
}

// Untrack price handler
func untrackPriceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	mu.Lock()
	delete(trackingItems, req.ID)
	mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Price tracking stopped",
	})
}

// Get tracked items handler
func getTrackedItemsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mu.RLock()
	items := make([]TrackingRequest, 0, len(trackingItems))
	for _, item := range trackingItems {
		items = append(items, item)
	}
	mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"items":   items,
	})
}

// Monitor prices continuously
func monitorPrices() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mu.RLock()
			for id, item := range trackingItems {
				go checkAndNotify(id, item)
			}
			mu.RUnlock()
		}
	}
}

func checkAndNotify(id string, item TrackingRequest) {
	log.Printf("Checking price for item %s: %s (target: %.2f)", id, item.URL, item.TargetPrice)
	priceString, currentPrice, err := scrapePrice(item.URL)
	if err != nil {
		log.Printf("Error checking price for %s: %v", id, err)
		return
	}

	log.Printf("Current price for %s: ₹%s (%.2f)", id, priceString, currentPrice)

	if currentPrice <= item.TargetPrice {
		log.Printf("Price target reached for %s! Current: %.2f, Target: %.2f", id, currentPrice, item.TargetPrice)
		alert := PriceAlert{
			ID:           id,
			URL:          item.URL,
			CurrentPrice: currentPrice,
			TargetPrice:  item.TargetPrice,
			PriceString:  priceString,
			Timestamp:    time.Now().Format(time.RFC3339),
		}

		// Send to all connected WebSocket clients
		mu.RLock()
		clientCount := len(clients)
		log.Printf("Sending alert to %d connected clients", clientCount)
		for client := range clients {
			select {
			case client.send <- alert:
				log.Printf("Alert sent to client successfully")
			default:
				log.Printf("Client channel full, closing connection")
				close(client.send)
				delete(clients, client)
			}
		}
		mu.RUnlock()

		log.Printf("Price alert sent for %s: ₹%s (target: ₹%.2f)", id, priceString, item.TargetPrice)

		// Stop monitoring this item after sending notification
		mu.Lock()
		delete(trackingItems, id)
		log.Printf("Stopped monitoring item %s after sending notification", id)
		mu.Unlock()
	} else {
		log.Printf("Price not yet at target for %s. Current: %.2f, Target: %.2f", id, currentPrice, item.TargetPrice)
	}
}
