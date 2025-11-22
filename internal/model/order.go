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
	RestaurantID int          `json:"restaurantId"`
	TableID      int          `json:"tableId"`
	Status       string       `json:"status"`
	Notes        string       `json:"notes"`
	TotalAmount  float64      `json:"totalAmount"`
	CreatedAt    string       `json:"createdAt"`
	Restaurant   Restaurant   `json:"restaurant"`
	Table        Table        `json:"table"`
	Plates       []OrderPlate `json:"orderPlates"`
	Drinks       []OrderDrink `json:"orderDrinks"`
}

type Restaurant struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Table struct {
	ID     int `json:"id"`
	Number int `json:"number"`
}

type OrderPlate struct {
	ID       int   `json:"id"`
	Quantity int   `json:"quantity"`
	Plate    Plate `json:"plate"`
}

type Plate struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type OrderDrink struct {
	ID       int   `json:"id"`
	Quantity int   `json:"quantity"`
	Drink    Drink `json:"drink"`
}

type Drink struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
