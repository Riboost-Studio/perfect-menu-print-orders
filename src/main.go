package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jung-kurt/gofpdf"
)

// --- Configuration Structures ---

const (
	configFile   = "config.json"
	printersFile = "printers.json"
	// apiURL       = "https://api.perfect-menu.it/api/printers"
	// wsURL        = "wss://ws.perfect-menu.it/agent"
	apiURL = "http://api.localhost/api/printers"
	wsURL  = "ws://ws.localhost/agent"
)

type Config struct {
	APIKey       string `json:"apiKey"`
	TenantID     int    `json:"tenantId"`
	RestaurantID int    `json:"restaurantId"`
}

type Printer struct {
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`
	Description  string `json:"description"`
	IsEnabled    bool   `json:"isEnabled"`
	TenantID     int    `json:"tenantId"`
	RestaurantID int    `json:"restaurantId,omitempty"`
	AgentKey     string `json:"agent_key,omitempty"` // Assigned by server
}

// --- Order Structures (Matching your JSON) ---

type OrderPayload struct {
	Success bool `json:"success"`
	Data    struct {
		Orders []Order `json:"orders"`
	} `json:"data"`
}

type Order struct {
	ID           int          `json:"id"`
	RestaurantID int          `json:"restaurantId"`
	TableID      int          `json:"tableId"`
	Status       string       `json:"status"`
	Notes        string       `json:"notes"`
	TotalAmount  float64      `json:"totalAmount"`
	CreatedAt    string       `json:"createdAt"`
	Restaurant   Restaurant   `json:"restaurant"`
	Table        Table        `json:"table"`
	Plates       []OrderPlate `json:"orderPlates"`
	Drinks       []OrderDrink `json:"orderDrinks"`
}

type Restaurant struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Table struct {
	ID     int    `json:"id"`
	Number string `json:"number"`
}

type OrderPlate struct {
	ID       int   `json:"id"`
	Quantity int   `json:"quantity"`
	Plate    Plate `json:"plate"`
}

type Plate struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type OrderDrink struct {
	ID       int   `json:"id"`
	Quantity int   `json:"quantity"`
	Drink    Drink `json:"drink"`
}

type Drink struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// --- WebSocket Messages ---

type WSMessage struct {
	Type     string          `json:"type"`
	AgentKey string          `json:"agent_key,omitempty"`
	Order    json.RawMessage `json:"order,omitempty"` // Keep raw to parse into specific structs
}

// --- Utility Functions ---

func detectLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no local IPv4 address found")
}

func probe(ip string, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func loadOrSetupConfig() (Config, error) {
	var config Config
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("--- Initial Setup ---")
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Enter Server API Key: ")
		config.APIKey, _ = reader.ReadString('\n')
		config.APIKey = strings.TrimSpace(config.APIKey)

		fmt.Print("Enter Tenant ID: ")
		fmt.Scanln(&config.TenantID)

		fmt.Print("Enter Restaurant ID: ")
		fmt.Scanln(&config.RestaurantID)

		data, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile(configFile, data, 0644)
		fmt.Println("Configuration saved.")
	} else {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return config, err
		}
		json.Unmarshal(data, &config)
	}
	return config, nil
}

func loadPrinters() ([]Printer, error) {
	if _, err := os.Stat(printersFile); os.IsNotExist(err) {
		return []Printer{}, nil
	}
	data, err := os.ReadFile(printersFile)
	if err != nil {
		return nil, err
	}
	var printers []Printer
	err = json.Unmarshal(data, &printers)
	return printers, err
}

func savePrinters(printers []Printer) error {
	data, err := json.MarshalIndent(printers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(printersFile, data, 0644)
}

// --- Discovery Logic ---

func discoverPrinters(config Config) []Printer {
	localIP, err := detectLocalIP()
	if err != nil {
		log.Println("Error detecting IP:", err)
		return nil
	}
	parts := strings.Split(localIP, ".")
	subnet := strings.Join(parts[:3], ".")
	fmt.Printf("Scanning subnet: %s.0/24\n", subnet)

	ipChan := make(chan string, 256)
	foundChan := make(chan string, 256)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipChan {
				if probe(ip, 9100) {
					foundChan <- ip
				}
			}
		}()
	}

	for i := 1; i <= 254; i++ {
		ipChan <- fmt.Sprintf("%s.%d", subnet, i)
	}
	close(ipChan)

	go func() {
		wg.Wait()
		close(foundChan)
	}()

	var newPrinters []Printer
	reader := bufio.NewReader(os.Stdin)

	for ip := range foundChan {
		fmt.Printf("Found printer at %s. Add this printer? (y/n): ", ip)
		ans, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) == "y" {
			p := Printer{
				IP:           ip,
				Port:         9100,
				IsEnabled:    true,
				TenantID:     config.TenantID,
				RestaurantID: config.RestaurantID,
			}

			fmt.Print("  Name (e.g., Kitchen): ")
			p.Name, _ = reader.ReadString('\n')
			p.Name = strings.TrimSpace(p.Name)

			fmt.Print("  Description (e.g., Thermal Printer): ")
			p.Description, _ = reader.ReadString('\n')
			p.Description = strings.TrimSpace(p.Description)

			newPrinters = append(newPrinters, p)
		}
	}
	return newPrinters
}

// --- API Registration ---

func registerPrinterOnServer(p *Printer, apiKey string) error {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API Error %d: %s", resp.StatusCode, string(body))
	}

	var responseMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseMap); err != nil {
		return err
	}

	if data, ok := responseMap["data"].(map[string]interface{}); ok {
		if key, ok := data["agent_key"].(string); ok {
			p.AgentKey = key
			return nil
		}
	}

	return fmt.Errorf("no agent_key found in response")
}

// --- WebSocket Agent Logic ---

func runAgent(p Printer, config Config) {
	header := http.Header{}
	header.Add("X-Api-Key", config.APIKey)

	log.Printf("[%s] Connecting to WebSocket...", p.Name)

	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			log.Printf("[%s] Connection failed: %v. Retrying in 5s...", p.Name, err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("[%s] Connected.", p.Name)
		handleConnection(conn, p)

		conn.Close()
		log.Printf("[%s] Disconnected. Reconnecting in 5s...", p.Name)
		time.Sleep(5 * time.Second)
	}
}

func handleConnection(conn *websocket.Conn, p Printer) {
	regMsg := WSMessage{
		Type:     "register",
		AgentKey: p.AgentKey,
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		log.Printf("[%s] Failed to send register: %v", p.Name, err)
		return
	}

	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("[%s] Read error: %v", p.Name, err)
			return
		}

		switch msg.Type {
		case "registered":
			log.Printf("[%s] Successfully registered with server.", p.Name)

		case "ping":
			log.Printf("[%s] Received ping, sending pong...", p.Name)
			conn.WriteJSON(WSMessage{Type: "pong", AgentKey: p.AgentKey})

		case "print":
			log.Printf("[%s] Received print order...", p.Name)
			// Pass the raw JSON to be parsed specifically
			handlePrintJob(p, msg.Order)

		case "unregister":
			log.Printf("[%s] Server requested unregister.", p.Name)
			return

		default:
			log.Printf("[%s] Unknown message type: %s", p.Name, msg.Type)
		}
	}
}

func handlePrintJob(p Printer, rawOrder json.RawMessage) {
	// 1. Parse the specific JSON structure
	var payload OrderPayload
	if err := json.Unmarshal(rawOrder, &payload); err != nil {
		log.Printf("[%s] Error parsing order JSON: %v", p.Name, err)
		return
	}

	if !payload.Success || len(payload.Data.Orders) == 0 {
		log.Printf("[%s] No valid orders in payload", p.Name)
		return
	}

	// Ensure tmp directory exists
	tmpDir := "tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		os.Mkdir(tmpDir, 0755)
	}

	// Process each order in the array
	for _, order := range payload.Data.Orders {
		log.Printf("[%s] Processing Order #%d for Table %s", p.Name, order.ID, order.Table.Number)

		// 2. Generate PDF
		pdfPath := filepath.Join(tmpDir, fmt.Sprintf("order_%d.pdf", order.ID))
		err := generateOrderPDF(order, pdfPath)
		if err != nil {
			log.Printf("[%s] Failed to generate PDF: %v", p.Name, err)
			continue
		}
		log.Printf("[%s] PDF generated: %s", p.Name, pdfPath)

		// 3. Send PDF to Printer
		if err := sendFileToPrinter(p, pdfPath); err != nil {
			log.Printf("[%s] Failed to send to printer: %v", p.Name, err)
		} else {
			log.Printf("[%s] Order sent successfully!", p.Name)
			
			// 4. Cleanup (Commented out as requested)
			// if err := os.Remove(pdfPath); err != nil {
			// 	log.Printf("[%s] Warning: Failed to delete tmp file: %v", p.Name, err)
			// } else {
			// 	log.Printf("[%s] Tmp file deleted.", p.Name)
			// }
		}
	}
}

func generateOrderPDF(order Order, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	// Header
	pdf.Cell(40, 10, order.Restaurant.Name)
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Order #%d", order.ID))
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("Table: %s", order.Table.Number))
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("Time: %s", order.CreatedAt))
	pdf.Ln(10)

	// Separator
	pdf.Cell(40, 10, "------------------------------------------------")
	pdf.Ln(10)

	// Plates
	pdf.SetFont("Arial", "B", 12)
	if len(order.Plates) > 0 {
		pdf.Cell(40, 10, "Food:")
		pdf.Ln(8)
		pdf.SetFont("Arial", "", 12)
		for _, item := range order.Plates {
			line := fmt.Sprintf("%dx %s", item.Quantity, item.Plate.Name)
			pdf.Cell(40, 10, line)
			pdf.Ln(6)
		}
		pdf.Ln(4)
	}

	// Drinks
	if len(order.Drinks) > 0 {
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(40, 10, "Drinks:")
		pdf.Ln(8)
		pdf.SetFont("Arial", "", 12)
		for _, item := range order.Drinks {
			line := fmt.Sprintf("%dx %s", item.Quantity, item.Drink.Name)
			pdf.Cell(40, 10, line)
			pdf.Ln(6)
		}
		pdf.Ln(4)
	}

	// Notes
	if order.Notes != "" {
		pdf.Ln(4)
		pdf.SetFont("Arial", "I", 11)
		pdf.Cell(40, 10, fmt.Sprintf("Notes: %s", order.Notes))
		pdf.Ln(10)
	}

	// Total
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, fmt.Sprintf("Total: %.2f", order.TotalAmount))

	return pdf.OutputFileAndClose(outputPath)
}

func sendFileToPrinter(p Printer, filePath string) error {
	// Read the generated file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	log.Printf("[%s] Sending %d bytes to %s:%d", p.Name, len(fileData), p.IP, p.Port)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", p.IP, p.Port), 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send file content to printer
	// Note: Port 9100 printers usually expect raw text, PCL, or PostScript. 
	// Sending a PDF binary directly might not work unless the printer explicitly supports PDF emulation.
	_, err = conn.Write(fileData)
	if err != nil {
		return err
	}
	return nil
}

// --- Main ---

func main() {
	// 1. Load Configuration
	config, err := loadOrSetupConfig()
	if err != nil {
		log.Fatal("Config error:", err)
	}

	// 2. Load Printers
	printers, err := loadPrinters()
	if err != nil {
		log.Println("Error loading printers, starting fresh.")
	}

	// 3. Discovery (if no printers found or forced)
	if len(printers) == 0 {
		fmt.Println("No printers configured. Starting discovery...")
		newPrinters := discoverPrinters(config)
		printers = append(printers, newPrinters...)
		savePrinters(printers)
	}

	// 4. Register Printers (Get Agent Keys)
	dirty := false
	for i := range printers {
		if printers[i].AgentKey == "" {
			fmt.Printf("Registering printer '%s' with server...\n", printers[i].Name)
			err := registerPrinterOnServer(&printers[i], config.APIKey)
			if err != nil {
				log.Printf("Failed to register %s: %v", printers[i].Name, err)
			} else {
				fmt.Printf("Success! Agent Key: %s\n", printers[i].AgentKey)
				dirty = true
			}
		}
	}
	if dirty {
		savePrinters(printers)
	}

	// 5. Start Agent for each Printer
	var wg sync.WaitGroup
	activePrinters := 0

	for _, p := range printers {
		if p.AgentKey != "" {
			activePrinters++
			wg.Add(1)
			// Run each printer agent in its own routine
			go func(printer Printer) {
				defer wg.Done()
				runAgent(printer, config)
			}(p)
		}
	}

	if activePrinters == 0 {
		fmt.Println("No printers are registered with an Agent Key. Exiting.")
		return
	}

	fmt.Printf("--- System Running. Controlling %d printers ---\n", activePrinters)

	// Wait for interrupt to exit cleanly
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down...")
}