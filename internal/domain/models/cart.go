package models

type CartItem struct {
	ProductID          int     `json:"product_id"`
	ProductName        string  `json:"product_name"`
	ProductDescription string  `json:"product_description"`
	ProductPrice       float64 `json:"product_price"`
	Quantity           int     `json:"quantity"`
}
