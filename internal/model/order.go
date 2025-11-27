package model

// --- Order Structures (Matching your JSON) ---
type OrderMessage struct {
	Type     string       `json:"type"`
	AgentKey string       `json:"agentKey"`
	Order    OrderPayload `json:"order"`
}

type OrderPayload struct {
	Success bool        `json:"success"`
	Data    PrinterData `json:"data"`
}

type PrinterData struct {
	Content  string   `json:"content,omitempty"`
	Copies   int      `json:"copies,omitempty"`
	Metadata Metadata `json:"metadata,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Type     string   `json:"type,omitempty"`
}

type Metadata struct {
	OrderId      int    `json:"orderId"`
	TenantId     int    `json:"tenantId"`
	RestaurantId int    `json:"restaurantId"`
	Category     string `json:"category"`
	TemplateName string `json:"templateName"`
	TemplateUsed int    `json:"templateUsed"`
	Timestamp    string `json:"timestamp"`
}