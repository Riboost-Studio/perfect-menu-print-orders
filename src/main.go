package main

import (
	"fmt"
	"os"
	"gopkg.in/yaml.v2"
	"net"
	"time"
)

// Printer represents a printer's basic info
 type Printer struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

// Config holds all discovered printers
 type Config struct {
	Printers []Printer `yaml:"printers"`
}

const configFile = "printers.yaml"

// scanNetwork tries to find printers on the local network (basic TCP port scan for 9100)
func scanNetwork() []Printer {
	printers := []Printer{}
	// Scan common local IP range (192.168.1.1-254)
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i)
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:9100", ip), 500*time.Millisecond)
		if err == nil {
			printers = append(printers, Printer{Name: fmt.Sprintf("Printer-%d", i), IP: ip})
			conn.Close()
		}
	}
	return printers
}

func main() {
	var cfg Config
	if _, err := os.Stat(configFile); err == nil {
		// Config file exists, read printers from it
		f, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Println("Error reading config file:", err)
			return
		}
		if err := yaml.Unmarshal(f, &cfg); err != nil {
			fmt.Println("Error parsing YAML:", err)
			return
		}
		fmt.Println("Printers loaded from config:")
		for _, p := range cfg.Printers {
			fmt.Printf("%s (%s)\n", p.Name, p.IP)
		}
		return
	}

	// Config file does not exist, scan network
	fmt.Println("Scanning network for printers...")
	cfg.Printers = scanNetwork()
	if len(cfg.Printers) == 0 {
		fmt.Println("No printers found.")
		return
	}
	// Save to YAML
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return
	}
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		fmt.Println("Error writing config file:", err)
		return
	}
	fmt.Println("Printers found and saved:")
	for _, p := range cfg.Printers {
		fmt.Printf("%s (%s)\n", p.Name, p.IP)
	}
}
