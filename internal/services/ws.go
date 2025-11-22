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
	"html/template"
	"bytes"
	"strconv"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"

	"github.com/gorilla/websocket"
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
		handleConnection(ctx, conn, p)

		conn.Close()
		log.Printf("[%s] Disconnected. Reconnecting in 5s...", p.Name)
		time.Sleep(5 * time.Second)
	}
}

func handleConnection(ctx context.Context, conn *websocket.Conn, p model.Printer) {
	regMsg := model.WSMessage{
		Type:     model.MessageTypeRegister,
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
		case model.MessageTypeRegistered:
			log.Printf("[%s] Successfully registered with server.", p.Name)

		case model.MessageTypePing:
			log.Printf("[%s] Received ping, sending pong...", p.Name)
			conn.WriteJSON(model.WSMessage{Type: model.MessageTypePong, AgentKey: p.AgentKey})

		case model.MessageTypeNewOrder:
			log.Printf("[%s] Received print order...", p.Name)
			// Pass the raw JSON to be parsed specifically
			handlePrintJob(ctx, conn, p, msg.Order)

		case model.MessageTypeUnregister:
			log.Printf("[%s] Server requested unregister.", p.Name)
			return

		default:
			log.Printf("[%s] Unknown message type: %s", p.Name, msg.Type)
		}
	}
}

func handlePrintJob(ctx context.Context, conn *websocket.Conn, p model.Printer, rawOrder json.RawMessage) {
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
		pdfPath := filepath.Join(tmpDir, fmt.Sprintf("%s_order_%d.pdf", p.AgentKey, order.ID))
		err := generateOrderPDF(ctx, order, pdfPath)
		if err != nil {
			log.Printf("[%s] Failed to generate PDF: %v", p.Name, err)
			continue
		}
		log.Printf("[%s] PDF generated: %s", p.Name, pdfPath)

		// 3. Send PDF to Printer
		if err := sendFileToPrinter(p, pdfPath); err != nil {
			log.Printf("[%s] Failed to send to printer: %v", p.Name, err)
			regMsg := model.WSMessage{
				Type:     model.MessageTypePrintFailed,
				AgentKey: p.AgentKey,
				Error:    err.Error(),
			}
			if err := conn.WriteJSON(regMsg); err != nil {
				log.Printf("[%s] Failed to send print_failed: %v", p.Name, err)
			}
		} else {
			regMsg := model.WSMessage{
				Type:     model.MessageTypePrinted,
				AgentKey: p.AgentKey,
			}
			if err := conn.WriteJSON(regMsg); err != nil {
				log.Printf("[%s] Failed to send printed: %v", p.Name, err)
				return
			}
			log.Printf("[%s] Order sent successfully!", p.Name)

			// 4. Cleanup (Commented out as requested)
			if err := os.Remove(pdfPath); err != nil {
				log.Printf("[%s] Warning: Failed to delete tmp file: %v", p.Name, err)
			} else {
				log.Printf("[%s] Tmp file deleted.", p.Name)
			}
		}
	}
}

// Define Helper functions for the template (formatting strings to money, dates, etc)
var templateFuncs = template.FuncMap{
	// Converts string "10.50" to float, then formats to "10.50"
	"formatMoney": func(amount string) string {
		val, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return "0.00"
		}
		return fmt.Sprintf("%.2f", val)
	},
	// Formats ISO date string to nice format
	"formatDate": func(dateStr string) string {
		// Assuming standard RFC3339 or similar from your JSON
		t, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
            // Fallback try simple date parsing or return original
			return dateStr
		}
		return t.Format("02/01/2006 15:04")
	},
}

func generateOrderPDF(ctx context.Context, order model.Order, outputPath string) error {
	// 1. Create a buffer to store the generated HTML
	var htmlBuffer bytes.Buffer

	// 2. Parse the HTML template
	templatePath := ctx.Value(model.TemplatePath).(string)
	templateFile := ctx.Value(model.TemplateFile).(string)
	tmplPath := filepath.Join(templatePath, templateFile)
	
	tmpl, err := template.New(templateFile).Funcs(templateFuncs).ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// 3. Execute the template with the order data
	if err := tmpl.Execute(&htmlBuffer, order); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// 4. Initialize the PDF Generator (wkhtmltopdf)
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return fmt.Errorf("failed to initialize pdf generator: %w", err)
	}

	// Set global options
	pdfg.Dpi.Set(300)
	pdfg.Orientation.Set(wkhtmltopdf.OrientationPortrait)
	pdfg.Grayscale.Set(false)
	pdfg.PageSize.Set(wkhtmltopdf.PageSizeA4)

	// Create a new Page from the HTML string
	page := wkhtmltopdf.NewPageReader(bytes.NewReader(htmlBuffer.Bytes()))
	
	// Optional: Enable local file access if you add images/logos later
	page.EnableLocalFileAccess.Set(true)
	
	pdfg.AddPage(page)

	// 5. Create the PDF
	if err := pdfg.Create(); err != nil {
		return fmt.Errorf("failed to create pdf: %w", err)
	}

	// 6. Write to file
	if err := pdfg.WriteFile(outputPath); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
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