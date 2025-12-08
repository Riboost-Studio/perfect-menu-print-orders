package model

// --- Configuration Structures ---

// Printer type constants
const (
	PrinterTypeThermal = "thermal"
	PrinterTypeInkjet  = "inkjet"
	PrinterTypeLaser   = "laser"
)

type Config struct {
	AppVersion   string `json:"appVersion"`
	APIKey       string `json:"apiKey"`
	TenantID     int    `json:"tenantId"`
	RestaurantID int    `json:"restaurantId"`
	ApiUrl       string `json:"apiUrl"`
	WsUrl        string `json:"wsUrl"`
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
	Type         string `json:"type,omitempty"`
	Size         int    `json:"size,omitempty"`
}
