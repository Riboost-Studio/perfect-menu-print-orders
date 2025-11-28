package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/utils"
)

// --- Discovery Logic ---

func DiscoverPrinters(config model.Config) []model.Printer {
	localIP, err := utils.DetectLocalIP()
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
				if utils.Probe(ip, 9100) {
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

	var newPrinters []model.Printer
	reader := bufio.NewReader(os.Stdin)

	for ip := range foundChan {
		fmt.Printf("Found printer at %s. Add this printer? (y/n): ", ip)
		ans, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(ans)) == "y" {
			p := model.Printer{
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

func RegisterPrinterOnServer(ctx context.Context, p *model.Printer, apiKey string) error {
	apiURL := ctx.Value(model.ContextAPIURL).(string) + "/api/printers"
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

func GetPrintersFromServer(ctx context.Context/*, p *model.Printer*/, apiKey string) ([]model.Printer, error) {
	apiURL := ctx.Value(model.ContextAPIURL).(string) + "/api/printers"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API Error %d: %s", resp.StatusCode, string(body))
	}

	var responseMap map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseMap); err != nil {
		return nil, err
	}

	if data, ok := responseMap["data"].(map[string]interface{}); ok {
		if printers, ok := data["printers"].([]interface{}); ok {
			var result []model.Printer
			for _, item := range printers {
				if pMap, ok := item.(map[string]interface{}); ok {
					pBytes, err := json.Marshal(pMap)
					if err != nil {
						continue
					}
					var p model.Printer
					if err := json.Unmarshal(pBytes, &p); err != nil {
						continue
					}
					result = append(result, p)
				}
			}
			return result, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("no agent_key found in response")
}


