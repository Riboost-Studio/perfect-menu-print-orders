package model

// --- Configuration Structures ---

type Config struct {
	APIKey       string `json:"apiKey"`
	TenantID     int    `json:"tenantId"`
	RestaurantID int    `json:"restaurantId"`
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
}