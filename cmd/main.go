package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/model"
	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/services"
	"github.com/Riboost-Studio/perfect-menu-print-orders/internal/utils"
)

const (
	appVersion   = "1.0.0"
	configFile   = "config/config.json"
	printersFile = "config/printers.json"
)

// --- Main ---

func main() {
	ctx := context.Background()
	ctx = context.WithValue(ctx, model.ContextAppName, "Perfect Menu Print Orders")
	ctx = context.WithValue(ctx, model.ContextAppVersion, appVersion)
	ctx = context.WithValue(ctx, model.ContextAppAuthor, "Riboost Studio")
	ctx = context.WithValue(ctx, model.ContextConfigFile, configFile)
	ctx = context.WithValue(ctx, model.ContextPrintersFile, printersFile)
	ctx = context.WithValue(ctx, model.TemplatePath, "templates")
	ctx = context.WithValue(ctx, model.TemplateFile, "order.html")

	// 0. Validate System Requirements
	fmt.Println("=== System Validation ===")
	if err := utils.ValidateSystemRequirements(); err != nil {
		log.Fatal("System validation failed:", err)
	}
	fmt.Println("=== System OK ===")
	fmt.Println()

	// 1. Load Configuration
	config, err := utils.LoadOrSetupConfig(ctx)
	if err != nil {
		log.Fatal("Config error:", err)
	}
	fmt.Printf("Configuration loaded: AppVersion=%s, API URL=%s, WS URL=%s\n", config.AppVersion, config.ApiUrl, config.WsUrl)
	ctx = context.WithValue(ctx, model.ContextAPIURL, config.ApiUrl)
	ctx = context.WithValue(ctx, model.ContextWSURL, config.WsUrl)

	// Sync Printers with Server
	serverPrinters, err := services.GetPrintersFromServer(ctx, config.APIKey)
	if err != nil {
		log.Println("Error getting printers from server:", err)
	}
	utils.SavePrinters(printersFile, serverPrinters)
	log.Printf("Synchronized %d printers from server.\n", len(serverPrinters))

	// 2. Load Printers
	printers, err := utils.LoadPrinters(ctx)
	if err != nil {
		log.Println("Error loading printers, starting fresh.")
	}

	// 3. Discovery (if no printers found or forced)
	if len(printers) == 0 {
		fmt.Println("No printers configured. Starting discovery...")
		newPrinters := services.DiscoverPrinters(config)
		printers = append(printers, newPrinters...)
		utils.SavePrinters(printersFile, printers)
	}

	// 4. Register Printers (Get Agent Keys)
	dirty := false
	for i := range printers {
		if printers[i].AgentKey == "" {
			fmt.Printf("Registering printer '%s' with server...\n", printers[i].Name)
			err := services.RegisterPrinterOnServer(ctx, &printers[i], config.APIKey)
			if err != nil {
				log.Printf("Failed to register %s: %v", printers[i].Name, err)
			} else {
				fmt.Printf("Success! Agent Key: %s\n", printers[i].AgentKey)
				dirty = true
			}
		}
	}
	if dirty {
		utils.SavePrinters(printersFile, printers)
	}

	// 5. Start Agent for each Printer
	var wg sync.WaitGroup
	activePrinters := 0

	for _, p := range printers {
		if p.AgentKey != "" {
			activePrinters++
			wg.Add(1)
			// Run each printer agent in its own routine
			go func(printer model.Printer) {
				defer wg.Done()
				services.RunAgent(ctx, printer, config)
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
