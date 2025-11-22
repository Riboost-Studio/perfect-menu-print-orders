package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jung-kurt/gofpdf"
	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
)

// --- WebSocket Agent Logic ---

func RunAgent(ctx context.Context, p model.Printer, config model.Config) {
	wsURL := ctx.Value(model.ContextWSURL).(string)
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

func handleConnection(conn *websocket.Conn, p model.Printer) {
	regMsg := model.WSMessage{
		Type:     "register",
		AgentKey: p.AgentKey,
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		log.Printf("[%s] Failed to send register: %v", p.Name, err)
		return
	}

	for {
		var msg model.WSMessage
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
			conn.WriteJSON(model.WSMessage{Type: "pong", AgentKey: p.AgentKey})

		case "print_order":
			log.Printf("[%s] Received print order...", p.Name)
			// Pass the raw JSON to be parsed specifically
			handlePrintJob(conn, p, msg.Order)

		case "unregister":
			log.Printf("[%s] Server requested unregister.", p.Name)
			return

		default:
			log.Printf("[%s] Unknown message type: %s", p.Name, msg.Type)
		}
	}
}

func handlePrintJob(conn *websocket.Conn, p model.Printer, rawOrder json.RawMessage) {
	// 1. Parse the specific JSON structure
	var payload model.OrderPayload
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
			regMsg := model.WSMessage{
				Type:     "printed",
				AgentKey: p.AgentKey,
			}
			if err := conn.WriteJSON(regMsg); err != nil {
				log.Printf("[%s] Failed to send printed: %v", p.Name, err)
				return
			}
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

func generateOrderPDF(order model.Order, outputPath string) error {
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
	if order.Notes != nil && *order.Notes != "" {
		pdf.Ln(4)
		pdf.SetFont("Arial", "I", 11)
		pdf.Cell(40, 10, fmt.Sprintf("Notes: %s", *order.Notes))
		pdf.Ln(10)
	}

	// Total
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, fmt.Sprintf("Total: %.2f", order.TotalAmount))

	return pdf.OutputFileAndClose(outputPath)
}

func sendFileToPrinter(p model.Printer, filePath string) error {
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