package utils

import (
	"context"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
)

// --- Utility Functions ---

func DetectLocalIP() (string, error) {
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

func Probe(ip string, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func LoadOrSetupConfig(ctx context.Context) (model.Config, error) {
	var config model.Config
	configFile := ctx.Value(model.ContextConfigFile).(string)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		config.AppVersion = ctx.Value(model.ContextAppVersion).(string)
		fmt.Println("--- Initial Setup ---")
		reader := bufio.NewReader(os.Stdin)

		apiUrl := "https://api.localhost"
		fmt.Printf("Enter API URL (default: %s): ", apiUrl)
		inputApiUrl, _ := reader.ReadString('\n')
		inputApiUrl = strings.TrimSpace(inputApiUrl)
		if inputApiUrl != "" {
			config.ApiUrl = inputApiUrl
		} else {
			config.ApiUrl = apiUrl
		}

		wsUrl := "wss://ws.localhost/agent"
		fmt.Printf("Enter WebSocket URL (default: %s): ", wsUrl)
		inputWsUrl, _ := reader.ReadString('\n')
		inputWsUrl = strings.TrimSpace(inputWsUrl)
		if inputWsUrl != "" {
			config.WsUrl = inputWsUrl
		} else {
			config.WsUrl = wsUrl
		}

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

func LoadPrinters(ctx context.Context) ([]model.Printer, error) {
	printersFile := ctx.Value(model.ContextPrintersFile).(string)
	if _, err := os.Stat(printersFile); os.IsNotExist(err) {
		return []model.Printer{}, nil
	}
	data, err := os.ReadFile(printersFile)
	if err != nil {
		return nil, err
	}
	var printers []model.Printer
	err = json.Unmarshal(data, &printers)
	return printers, err
}

func SavePrinters(printersFile string, printers []model.Printer) error {
	data, err := json.MarshalIndent(printers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(printersFile, data, 0644)
}