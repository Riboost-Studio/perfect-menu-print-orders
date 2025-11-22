package model

// --- Order Structures (Matching your JSON) ---
type OrderPayload struct {
	Success bool `json:"success"`
	Data    struct {
		Orders []Order `json:"orders"`
	} `json:"data"`
}

type Order struct {
	ID           int          `json:"id"`
	NOrder       int          `json:"nOrder"`
	Year         int          `json:"year"`
	RestaurantID int          `json:"restaurantId"`
	TableID      *int         `json:"tableId"`
	Status       string       `json:"status"`
	Notes        *string      `json:"notes"`
	TotalAmount  string       `json:"totalAmount"`
	OrderDate    string       `json:"orderDate"`
	CreatedAt    string       `json:"createdAt"`
	UpdatedAt    string       `json:"updatedAt"`
	Restaurant   Restaurant   `json:"restaurant"`
	Table        *Table       `json:"table"`
	Plates       []OrderPlate `json:"orderPlates"`
	Drinks       []OrderDrink `json:"orderDrinks"`
}

type Restaurant struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
	TenantID    int    `json:"tenantId"`
}

type Table struct {
	ID         int     `json:"id"`
	Number     int     `json:"number"`
	Capacity   int     `json:"capacity"`
	Status     string  `json:"status"`
	MenuUrl    *string `json:"menuUrl"`
	QrCodeData *string `json:"qrCodeData"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type OrderPlate struct {
	ID                 int                 `json:"id"`
	OrderID            int                 `json:"orderId"`
	LineID             int                 `json:"lineId"`
	PlateID            int                 `json:"plateId"`
	Quantity           int                 `json:"quantity"`
	UnitPrice          string              `json:"unitPrice"`
	Subtotal           string              `json:"subtotal"`
	Notes              *string             `json:"notes"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
	Plate              Plate               `json:"plate"`
	OrderPlateProducts []OrderPlateProduct `json:"orderPlateProducts"`
}

type Plate struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
}

type OrderDrink struct {
	ID        int     `json:"id"`
	OrderID   int     `json:"orderId"`
	LineID    int     `json:"lineId"`
	DrinkID   int     `json:"drinkId"`
	Quantity  int     `json:"quantity"`
	UnitPrice string  `json:"unitPrice"`
	Subtotal  string  `json:"subtotal"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	Drink     Drink   `json:"drink"`
}

type Drink struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
}

// OrderPlateProduct represents a product added to an order plate
type OrderPlateProduct struct {
	ID           int     `json:"id"`
	OrderPlateID int     `json:"orderPlateId"`
	ProductID    int     `json:"productId"`
	Quantity     int     `json:"quantity"`
	Price        string  `json:"price"`
	Subtotal     string  `json:"subtotal"`
	Notes        *string `json:"notes"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
	Product      Product `json:"product"`
}

// Product represents a product that can be added to plates
type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
}
