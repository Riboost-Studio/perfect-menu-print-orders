package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"image"
	"image/png"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
	"github.com/gorilla/websocket"
)

// Printer type constants
const (
	PrinterTypeThermal = "thermal"
	PrinterTypeInkjet  = "inkjet"
	PrinterTypeLaser   = "laser"
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
			pongMsg := model.WSMessageTypePong{
				Type:      model.MessageTypePong,
				Timestamp: time.Now().Unix(),
			}
			conn.WriteJSON(pongMsg)

		case model.MessageTypeNewOrder:
			log.Printf("[%s] Received print order...", p.Name)
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
	
	// Ensure we have content to print
	if payload.Data.Content == "" {
		log.Printf("[%s] Received empty content, skipping.", p.Name)
		return
	}

	log.Printf("[%s] Processing Order ID: %d (Type: %s)", p.Name, payload.Data.Metadata.OrderId, p.Type)

	// Ensure tmp directory exists
	tmpDir := "tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		os.Mkdir(tmpDir, 0755)
	}

	// Determine number of copies (default to 1 if 0)
	copies := payload.Data.Copies
	if copies < 1 {
		copies = 1
	}

	// 2. Generate IMG
	fileName := fmt.Sprintf("%s_order_%d_%d.png", p.AgentKey, payload.Data.Metadata.OrderId, time.Now().Unix())
	imgPath := filepath.Join(tmpDir, fileName)

	err := generateOrderImage(ctx, payload.Data.Content, imgPath)
	if err != nil {
		log.Printf("[%s] Failed to generate IMG: %v", p.Name, err)
		
		failMsg := model.WSMessage{
			Type:     model.MessageTypePrintFailed,
			AgentKey: p.AgentKey,
			Error:    err.Error(),
		}
		conn.WriteJSON(failMsg)
		return
	}
	log.Printf("[%s] IMG generated: %s", p.Name, imgPath)

	// 3. Send IMG to Printer (Loop for copies)
	success := true
	for i := 0; i < copies; i++ {
		log.Printf("[%s] Printing copy %d of %d", p.Name, i+1, copies)
		if err := sendFileToPrinter(p, imgPath); err != nil {
			log.Printf("[%s] Failed to send to printer: %v", p.Name, err)
			success = false
			
			failMsg := model.WSMessageTypePrintFailed{
				Type:     model.MessageTypePrintFailed,
				AgentKey: p.AgentKey,
				OrderID:  payload.Data.Metadata.OrderId,
				Error:    err.Error(),
			}
			conn.WriteJSON(failMsg)
			break
		}
	}

	if success {
		regMsg := model.WSMessage{
			Type:     model.MessageTypePrinted,
			AgentKey: p.AgentKey,
		}
		if err := conn.WriteJSON(regMsg); err != nil {
			log.Printf("[%s] Failed to send printed confirmation: %v", p.Name, err)
		}
		log.Printf("[%s] Order sent successfully!", p.Name)
	}

	// 4. Cleanup
	if err := os.Remove(imgPath); err != nil {
		log.Printf("[%s] Warning: Failed to delete tmp file: %v", p.Name, err)
	} else {
		log.Printf("[%s] Tmp file deleted.", p.Name)
	}
}

func generateOrderImage(ctx context.Context, htmlContent string, outputPath string) error {
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

	var pngBytes []byte

	err := chromedp.Run(cdpCtx,
		chromedp.Navigate("data:text/html,"+urlEncode(htmlContent)),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, err := page.CaptureScreenshot().
				WithCaptureBeyondViewport(true).
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

	if err := os.WriteFile(outputPath, pngBytes, 0644); err != nil {
		return fmt.Errorf("failed saving image: %w", err)
	}

	return nil
}

func urlEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// --- MAIN DISPATCHER ---
func sendFileToPrinter(p model.Printer, filePath string) error {
	// Normalize printer type to lowercase
	printerType := strings.ToLower(strings.TrimSpace(p.Type))
	
	switch printerType {
	case PrinterTypeThermal:
		return sendToThermalPrinter(p, filePath)
	
	case PrinterTypeInkjet, PrinterTypeLaser:
		return sendToSystemPrinter(p, filePath)
	
	case "":
		// Default to thermal for backward compatibility
		log.Printf("[%s] Warning: No printer type specified, defaulting to thermal", p.Name)
		return sendToThermalPrinter(p, filePath)
	
	default:
		return fmt.Errorf("unsupported printer type: %s (must be thermal, inkjet, or laser)", p.Type)
	}
}

// --- THERMAL PRINTER (ESC/POS) ---
func sendToThermalPrinter(p model.Printer, filePath string) error {
	log.Printf("[%s] Using thermal printer mode (ESC/POS)", p.Name)
	
	// Load PNG
	imgFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	img, err := png.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("failed to decode PNG: %w", err)
	}

	// Resize to thermal printer width (384px standard)
	img = resizeToWidth(img, 384)

	// Convert to ESC/POS raster
	escposData, err := convertImageToESCPOS(img)
	if err != nil {
		return fmt.Errorf("ESC/POS conversion failed: %w", err)
	}

	// Build complete print job
	var printJob []byte
	
	// Initialize printer
	printJob = append(printJob, 0x1B, 0x40) // ESC @
	
	// Add the image data
	printJob = append(printJob, escposData...)
	
	// Feed paper and cut
	printJob = append(printJob, 0x1B, 0x64, 0x03) // ESC d 3 - feed 3 lines
	printJob = append(printJob, 0x1D, 0x56, 0x41, 0x00) // GS V A 0 - partial cut

	log.Printf("[%s] Sending %d bytes to %s:%d", p.Name, len(printJob), p.IP, p.Port)

	// Send to printer via raw TCP
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

// --- INKJET/LASER PRINTER (System Print Spooler) ---
func sendToSystemPrinter(p model.Printer, filePath string) error {
	log.Printf("[%s] Using system printer mode (%s)", p.Name, p.Type)
	
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin": // macOS
		// Try using the printer name if configured in system
		if p.Name != "" {
			cmd = exec.Command("lpr", "-P", p.Name, filePath)
		} else {
			// Fallback to IPP
			ippURI := fmt.Sprintf("ipp://%s/ipp/print", p.IP)
			cmd = exec.Command("lpr", "-H", p.IP, filePath)
			log.Printf("[%s] Using IPP URI: %s", p.Name, ippURI)
		}
		
	case "linux":
		// Try using the printer name if configured in system
		if p.Name != "" {
			cmd = exec.Command("lp", "-d", p.Name, filePath)
		} else {
			// Fallback to IPP
			ippURI := fmt.Sprintf("ipp://%s/ipp/print", p.IP)
			cmd = exec.Command("lp", "-d", ippURI, filePath)
			log.Printf("[%s] Using IPP URI: %s", p.Name, ippURI)
		}
		
	case "windows":
		// Windows printing
		if p.Name != "" {
			// Use mspaint for simple printing (or use a better method)
			cmd = exec.Command("mspaint.exe", "/pt", filePath, p.Name)
		} else {
			return fmt.Errorf("Windows printer requires printer name to be configured in system")
		}
		
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	// Execute print command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("print command failed: %w, output: %s", err, string(output))
	}
	
	log.Printf("[%s] Sent to system print spooler", p.Name)
	return nil
}

// --- ESC/POS CONVERSION ---
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

// --- IMAGE RESIZING ---
func resizeToWidth(src image.Image, targetWidth int) image.Image {
    bounds := src.Bounds()
    w := bounds.Dx()
    h := bounds.Dy()

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