package models

import "time"

const (
	RoleSeller     = "seller"
	RoleSellerHead = "seller_head"
	RoleAdmin      = "admin"

	PaymentCash = "cash"
	PaymentCard = "card"

	CategoryDrinks = "Напої"
	CategoryFood   = "Їжа"
	CategorySweets = "Солодощі"
)

type User struct {
	UserID    int64
	Username  string
	FullName  string
	Role      string
	CreatedAt time.Time
}

type Product struct {
	ID        int64
	Name      string
	Category  string
	CostPrice float64
	SellPrice float64
	Unit      string
	Stock     float64
	IsActive  bool
	CreatedAt time.Time
}

type Sale struct {
	ID            int64
	UserID        int64
	Total         float64
	CostTotal     float64
	PaymentMethod string
	CreatedAt     time.Time
}

type SaleItem struct {
	ID        int64
	SaleID    int64
	ProductID int64
	Name      string
	Qty       float64
	SellPrice float64
	CostPrice float64
}

type OrderItem struct {
	ProductID int64
	Name      string
	Category  string
	Qty       float64
	SellPrice float64
	CostPrice float64
	Unit      string
}

type UserSalesSummary struct {
	CashTotal  float64
	CardTotal  float64
	GrandTotal float64
	Checks     int
	Items      []DailyReportItem
}

type DailyReport struct {
	Date         time.Time
	CashRevenue  float64
	CardRevenue  float64
	TotalRevenue float64
	CostTotal    float64
	Profit       float64
	Checks       int
	Items        []DailyReportItem
	Sellers      []SellerSummary
}

type DailyReportItem struct {
	ProductID int64
	Name      string
	Category  string
	Unit      string
	Qty       float64
	Revenue   float64
	Cost      float64
}

// SellerSummary is one seller's share of a day's sales.
type SellerSummary struct {
	UserID int64
	Name   string
	Checks int
	Cash   float64
	Card   float64
	Total  float64
}

func Categories() []string {
	return []string{CategoryDrinks, CategoryFood, CategorySweets}
}

// PurchaseItem is a position from the admin-managed list that staff can tap
// when composing a purchase request.
type PurchaseItem struct {
	ID        int64
	Name      string
	Unit      string
	CreatedAt time.Time
}
