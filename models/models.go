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
}

type DailyReport struct {
	Date         time.Time
	CashRevenue  float64
	CardRevenue  float64
	TotalRevenue float64
	CostTotal    float64
	Profit       float64
	Items        []DailyReportItem
}

type DailyReportItem struct {
	ProductID int64
	Name      string
	Qty       float64
}

func Categories() []string {
	return []string{CategoryDrinks, CategoryFood, CategorySweets}
}
