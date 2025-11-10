// main.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Printer struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

const printersFile = "printers.json"

// --- Detect local IPv4 ---
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

// --- Probe a TCP port ---
func probe(ip string, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// --- Send raw print job (JetDirect) ---
func sendRaw(printer Printer, filePath string) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", printer.IP, printer.Port), 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(conn, f)
	return err
}

// --- Load printers from JSON ---
func loadPrinters() ([]Printer, error) {
	data, err := os.ReadFile(printersFile)
	if err != nil {
		return nil, err
	}
	var printers []Printer
	err = json.Unmarshal(data, &printers)
	return printers, err
}

// --- Save printers to JSON ---
func savePrinters(printers []Printer) error {
	data, err := json.MarshalIndent(printers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(printersFile, data, 0644)
}

// --- Scan network for printers ---
func discoverPrinters() []Printer {
	localIP, err := detectLocalIP()
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 {
		fmt.Println("Unexpected IP format")
		return nil
	}
	subnet := strings.Join(parts[:3], ".")
	fmt.Println("Scanning subnet:", subnet+".0/24")

	var wg sync.WaitGroup
	ipChan := make(chan string, 256)
	foundChan := make(chan string, 256)

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
		ip := fmt.Sprintf("%s.%d", subnet, i)
		if ip != localIP {
			ipChan <- ip
		}
	}
	close(ipChan)

	go func() {
		wg.Wait()
		close(foundChan)
	}()

	var printers []Printer
	reader := bufio.NewReader(os.Stdin)
	for ip := range foundChan {
		fmt.Printf("Printer found at %s. Add it? (y/n): ", ip)
		answer, _ := reader.ReadString('\n')
		if strings.HasPrefix(strings.ToLower(answer), "y") {
			printers = append(printers, Printer{IP: ip, Port: 9100})
		}
	}

	return printers
}

func main() {
	var printers []Printer
	var err error

	if _, err = os.Stat(printersFile); err == nil {
		printers, err = loadPrinters()
		if err == nil && len(printers) > 0 {
			fmt.Println("Loaded saved printers:")
			for _, p := range printers {
				fmt.Printf("- %s:%d\n", p.IP, p.Port)
			}
		}
	}

	if len(printers) == 0 {
		printers = discoverPrinters()
		if len(printers) > 0 {
			savePrinters(printers)
			fmt.Println("Printers saved to", printersFile)
		} else {
			fmt.Println("No printers saved.")
			return
		}
	}

	// Ask file to print
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter file path to send to all printers: ")
	filePath, _ := reader.ReadString('\n')
	filePath = strings.TrimSpace(filePath)

	for _, p := range printers {
		fmt.Printf("Sending file to %s:%d...\n", p.IP, p.Port)
		err := sendRaw(p, filePath)
		if err != nil {
			fmt.Println("  ❌ Error:", err)
		} else {
			fmt.Println("  ✅ Sent successfully.")
		}
	}
}
