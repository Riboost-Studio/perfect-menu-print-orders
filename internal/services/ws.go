package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"image"
	"image/png"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
	"github.com/gorilla/websocket"
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

		// 2. Generate IMG
		imgPath := filepath.Join(tmpDir, fmt.Sprintf("%s_order_%d.png", p.AgentKey, order.ID))
		err := generateOrderImage(ctx, order, imgPath)
		if err != nil {
			log.Printf("[%s] Failed to generate IMG: %v", p.Name, err)
			continue
		}
		log.Printf("[%s] IMG generated: %s", p.Name, imgPath)

		// 3. Send IMG to Printer
		if err := sendFileToPrinter(p, imgPath); err != nil {
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
			if err := os.Remove(imgPath); err != nil {
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

func generateOrderImage(ctx context.Context, order model.Order, outputPath string) error {
	// Render HTML from template
	var htmlBuffer bytes.Buffer

	templatePath := ctx.Value(model.TemplatePath).(string)
	templateFile := ctx.Value(model.TemplateFile).(string)
	tmplPath := filepath.Join(templatePath, templateFile)

	tmpl, err := template.New(templateFile).Funcs(templateFuncs).ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := tmpl.Execute(&htmlBuffer, order); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var cdpCtx context.Context
	var cancel context.CancelFunc

	// macOS: force Chrome path
	if runtime.GOOS == "darwin" {
		opts := append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
		)

		allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
		cdpCtx, cancel = chromedp.NewContext(allocCtx)
		defer allocCancel()
		defer cancel()
	} else {
		cdpCtx, cancel = chromedp.NewContext(context.Background())
		defer cancel()
	}

	html := htmlBuffer.String()
	var pngBytes []byte

	err = chromedp.Run(cdpCtx,
		// Load HTML directly using data URL
		chromedp.Navigate("data:text/html,"+urlEncode(html)),

		// Wait for the page to render
		chromedp.Sleep(300*time.Millisecond),

		// Capture full-page PNG screenshot
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, err := page.CaptureScreenshot().
				WithCaptureBeyondViewport(true). // capture full height
				Do(ctx)
			if err != nil {
				return err
			}

			pngBytes = buf
			return nil
		}),
	)

	if err != nil {
		return fmt.Errorf("failed generating image: %w", err)
	}

	// Save PNG
	if err := os.WriteFile(outputPath, pngBytes, 0644); err != nil {
		return fmt.Errorf("failed saving image: %w", err)
	}

	return nil
}


// Helper for encoding HTML into a data URL
func urlEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func convertImageToESCPOS(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// ESC/POS width must be divisible by 8
	if width%8 != 0 {
		width = width - (width % 8)
	}

	rowBytes := width / 8
	raster := make([]byte, rowBytes*height)

	// Convert to 1-bit
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			gray := (r + g + b) / 3

			bit := uint8(0)
			if gray < 0x8000 { // threshold
				bit = 1
			}

			byteIndex := y*rowBytes + x/8
			bitPos := 7 - (x % 8)

			if bit == 1 {
				raster[byteIndex] |= (1 << bitPos)
			}
		}
	}

	// ESC/POS header: GS v 0
	header := []byte{
		0x1D, 0x76, 0x30, 0x00,
		byte(rowBytes), byte(rowBytes >> 8),
		byte(height), byte(height >> 8),
	}

	return append(header, raster...), nil
}


func sendFileToPrinter(p model.Printer, filePath string) error {
	// --- Load PNG ---
	imgFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	img, err := png.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("failed to decode PNG: %w", err)
	}

	img = resizeToWidth(img, 384)

	// --- Convert to ESC/POS raster ---
	escposData, err := convertImageToESCPOS(img)
	if err != nil {
		return fmt.Errorf("ESC/POS conversion failed: %w", err)
	}

	// --- Build complete print job ---
	var printJob []byte
	
	// Initialize printer
	printJob = append(printJob, 0x1B, 0x40) // ESC @
	
	// Add the image data
	printJob = append(printJob, escposData...)
	
	// Feed paper and cut (optional but recommended)
	printJob = append(printJob, 0x1B, 0x64, 0x03) // ESC d 3 - feed 3 lines
	printJob = append(printJob, 0x1D, 0x56, 0x41, 0x00) // GS V A 0 - partial cut

	log.Printf("[%s] Sending %d bytes to %s:%d",
		p.Name, len(printJob), p.IP, p.Port)

	// --- Send to printer ---
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", p.IP, p.Port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(printJob)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Give printer time to process
	time.Sleep(500 * time.Millisecond)

	return nil
}

func resizeToWidth(src image.Image, targetWidth int) image.Image {
    bounds := src.Bounds()
    w := bounds.Dx()
    h := bounds.Dy()

    // ESC/POS standard width is normally 384px
    scale := float64(targetWidth) / float64(w)
    newHeight := int(float64(h) * scale)

    dst := image.NewRGBA(image.Rect(0, 0, targetWidth, newHeight))

    for y := 0; y < newHeight; y++ {
        for x := 0; x < targetWidth; x++ {
            sx := int(float64(x) / scale)
            sy := int(float64(y) / scale)
            dst.Set(x, y, src.At(sx, sy))
        }
    }

    return dst
}
