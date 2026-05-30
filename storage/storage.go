package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

const sqliteDateTimeLayout = "2006-01-02 15:04:05"

type Storage struct {
	db *sql.DB
}

func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) Init(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS users (
user_id INTEGER PRIMARY KEY,
username TEXT,
full_name TEXT,
role TEXT NOT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`,
		`CREATE TABLE IF NOT EXISTS products (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL,
category TEXT NOT NULL,
cost_price REAL NOT NULL,
sell_price REAL NOT NULL,
unit TEXT NOT NULL,
stock REAL NOT NULL DEFAULT 0,
is_active INTEGER NOT NULL DEFAULT 1,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`,
		`CREATE TABLE IF NOT EXISTS sales (
id INTEGER PRIMARY KEY AUTOINCREMENT,
user_id INTEGER NOT NULL,
total REAL NOT NULL,
cost_total REAL NOT NULL,
payment_method TEXT NOT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
FOREIGN KEY(user_id) REFERENCES users(user_id)
);`,
		`CREATE TABLE IF NOT EXISTS sale_items (
id INTEGER PRIMARY KEY AUTOINCREMENT,
sale_id INTEGER NOT NULL,
product_id INTEGER NOT NULL,
name TEXT NOT NULL,
qty REAL NOT NULL,
sell_price REAL NOT NULL,
cost_price REAL NOT NULL,
FOREIGN KEY(sale_id) REFERENCES sales(id),
FOREIGN KEY(product_id) REFERENCES products(id)
);`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) SeedAdmins(ctx context.Context, adminIDs []int64) error {
	for _, adminID := range adminIDs {
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO users (user_id, username, full_name, role)
VALUES (?, '', ?, ?)
ON CONFLICT(user_id) DO UPDATE SET role = excluded.role
`, adminID, fmt.Sprintf("Адміністратор %d", adminID), models.RoleAdmin); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetUser(ctx context.Context, userID int64) (models.User, error) {
	var user models.User
	var createdAt string
	row := s.db.QueryRowContext(ctx, `SELECT user_id, username, full_name, role, created_at FROM users WHERE user_id = ?`, userID)
	if err := row.Scan(&user.UserID, &user.Username, &user.FullName, &user.Role, &createdAt); err != nil {
		return models.User{}, err
	}
	user.CreatedAt = parseDBTime(createdAt)
	return user, nil
}

func (s *Storage) TouchUser(ctx context.Context, user models.User) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET username = ?, full_name = ? WHERE user_id = ?`, user.Username, user.FullName, user.UserID)
	return err
}

func (s *Storage) ListAdmins(ctx context.Context) ([]models.User, error) {
	return s.listUsersByRoles(ctx, models.RoleAdmin)
}

func (s *Storage) listUsersByRoles(ctx context.Context, roles ...string) ([]models.User, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(roles)), ",")
	args := make([]any, len(roles))
	for i, role := range roles {
		args[i] = role
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, username, full_name, role, created_at FROM users WHERE role IN (`+placeholders+`) ORDER BY full_name, user_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var createdAt string
		if err := rows.Scan(&user.UserID, &user.Username, &user.FullName, &user.Role, &createdAt); err != nil {
			return nil, err
		}
		user.CreatedAt = parseDBTime(createdAt)
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Storage) CreateProduct(ctx context.Context, product models.Product) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO products (name, category, cost_price, sell_price, unit, stock, is_active)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, product.Name, product.Category, product.CostPrice, product.SellPrice, product.Unit, product.Stock, boolToInt(product.IsActive))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Storage) GetProduct(ctx context.Context, productID int64) (models.Product, error) {
	var product models.Product
	var createdAt string
	var active int
	row := s.db.QueryRowContext(ctx, `SELECT id, name, category, cost_price, sell_price, unit, stock, is_active, created_at FROM products WHERE id = ?`, productID)
	if err := row.Scan(&product.ID, &product.Name, &product.Category, &product.CostPrice, &product.SellPrice, &product.Unit, &product.Stock, &active, &createdAt); err != nil {
		return models.Product{}, err
	}
	product.IsActive = active == 1
	product.CreatedAt = parseDBTime(createdAt)
	return product, nil
}

func (s *Storage) ListProducts(ctx context.Context, includeInactive bool) ([]models.Product, error) {
	query := `SELECT id, name, category, cost_price, sell_price, unit, stock, is_active, created_at FROM products`
	if !includeInactive {
		query += ` WHERE is_active = 1`
	}
	query += ` ORDER BY category, name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (s *Storage) ListProductsByCategory(ctx context.Context, category string) ([]models.Product, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, category, cost_price, sell_price, unit, stock, is_active, created_at
FROM products
WHERE is_active = 1 AND category = ?
ORDER BY name
`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (s *Storage) UpdateProductName(ctx context.Context, productID int64, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET name = ? WHERE id = ?`, name, productID)
	return err
}

func (s *Storage) UpdateProductCostPrice(ctx context.Context, productID int64, costPrice float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET cost_price = ? WHERE id = ?`, costPrice, productID)
	return err
}

func (s *Storage) UpdateProductSellPrice(ctx context.Context, productID int64, sellPrice float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET sell_price = ? WHERE id = ?`, sellPrice, productID)
	return err
}

func (s *Storage) UpdateProductStock(ctx context.Context, productID int64, stock float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET stock = ? WHERE id = ?`, stock, productID)
	return err
}

func (s *Storage) IncreaseProductStock(ctx context.Context, productID int64, delta float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET stock = stock + ? WHERE id = ?`, delta, productID)
	return err
}

func (s *Storage) ListLowStockProducts(ctx context.Context, threshold float64) ([]models.Product, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, category, cost_price, sell_price, unit, stock, is_active, created_at
FROM products
WHERE is_active = 1 AND stock <= ?
ORDER BY stock ASC, name ASC
`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (s *Storage) RecordSale(ctx context.Context, userID int64, paymentMethod string, items []models.OrderItem) (models.Sale, []models.SaleItem, error) {
	if len(items) == 0 {
		return models.Sale{}, nil, errors.New("кошик порожній")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Sale{}, nil, err
	}
	defer tx.Rollback()

	var total float64
	var costTotal float64
	recordedItems := make([]models.SaleItem, 0, len(items))
	for _, item := range items {
		var product models.Product
		var active int
		var createdAt string
		row := tx.QueryRowContext(ctx, `SELECT id, name, category, cost_price, sell_price, unit, stock, is_active, created_at FROM products WHERE id = ?`, item.ProductID)
		if err := row.Scan(&product.ID, &product.Name, &product.Category, &product.CostPrice, &product.SellPrice, &product.Unit, &product.Stock, &active, &createdAt); err != nil {
			return models.Sale{}, nil, err
		}
		if active != 1 {
			return models.Sale{}, nil, fmt.Errorf("товар %q неактивний", product.Name)
		}
		if product.Stock < item.Qty {
			return models.Sale{}, nil, fmt.Errorf("недостатньо залишку для товару %q", product.Name)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET stock = stock - ? WHERE id = ?`, item.Qty, item.ProductID); err != nil {
			return models.Sale{}, nil, err
		}
		total += item.Qty * product.SellPrice
		costTotal += item.Qty * product.CostPrice
		recordedItems = append(recordedItems, models.SaleItem{
			ProductID: item.ProductID,
			Name:      product.Name,
			Qty:       item.Qty,
			SellPrice: product.SellPrice,
			CostPrice: product.CostPrice,
		})
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO sales (user_id, total, cost_total, payment_method) VALUES (?, ?, ?, ?)`, userID, total, costTotal, paymentMethod)
	if err != nil {
		return models.Sale{}, nil, err
	}
	saleID, err := result.LastInsertId()
	if err != nil {
		return models.Sale{}, nil, err
	}

	for i := range recordedItems {
		recordedItems[i].SaleID = saleID
		insertResult, err := tx.ExecContext(ctx, `
INSERT INTO sale_items (sale_id, product_id, name, qty, sell_price, cost_price)
VALUES (?, ?, ?, ?, ?, ?)
`, saleID, recordedItems[i].ProductID, recordedItems[i].Name, recordedItems[i].Qty, recordedItems[i].SellPrice, recordedItems[i].CostPrice)
		if err != nil {
			return models.Sale{}, nil, err
		}
		recordedItems[i].ID, _ = insertResult.LastInsertId()
	}

	if err := tx.Commit(); err != nil {
		return models.Sale{}, nil, err
	}

	return models.Sale{ID: saleID, UserID: userID, Total: total, CostTotal: costTotal, PaymentMethod: paymentMethod, CreatedAt: time.Now()}, recordedItems, nil
}

func (s *Storage) GetUserSalesSummary(ctx context.Context, userID int64, date time.Time) (models.UserSalesSummary, error) {
	start, end := dayBounds(date)
	var summary models.UserSalesSummary
	row := s.db.QueryRowContext(ctx, `
SELECT
COALESCE(SUM(CASE WHEN payment_method = ? THEN total END), 0),
COALESCE(SUM(CASE WHEN payment_method = ? THEN total END), 0),
COALESCE(SUM(total), 0),
COUNT(*)
FROM sales
WHERE user_id = ? AND created_at >= ? AND created_at < ?
`, models.PaymentCash, models.PaymentCard, userID, formatDBTime(start), formatDBTime(end))
	if err := row.Scan(&summary.CashTotal, &summary.CardTotal, &summary.GrandTotal, &summary.Checks); err != nil {
		return summary, err
	}
	return summary, nil
}

func (s *Storage) GetDailyReport(ctx context.Context, date time.Time) (models.DailyReport, error) {
	start, end := dayBounds(date)
	report := models.DailyReport{Date: start}

	row := s.db.QueryRowContext(ctx, `
SELECT
COALESCE(SUM(CASE WHEN payment_method = ? THEN total END), 0),
COALESCE(SUM(CASE WHEN payment_method = ? THEN total END), 0),
COALESCE(SUM(total), 0),
COALESCE(SUM(cost_total), 0)
FROM sales
WHERE created_at >= ? AND created_at < ?
`, models.PaymentCash, models.PaymentCard, formatDBTime(start), formatDBTime(end))
	if err := row.Scan(&report.CashRevenue, &report.CardRevenue, &report.TotalRevenue, &report.CostTotal); err != nil {
		return report, err
	}
	report.Profit = report.TotalRevenue - report.CostTotal

	rows, err := s.db.QueryContext(ctx, `
SELECT si.product_id, si.name, COALESCE(SUM(si.qty), 0)
FROM sale_items si
JOIN sales s ON s.id = si.sale_id
WHERE s.created_at >= ? AND s.created_at < ?
GROUP BY si.product_id, si.name
ORDER BY si.name
`, formatDBTime(start), formatDBTime(end))
	if err != nil {
		return report, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.DailyReportItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Qty); err != nil {
			return report, err
		}
		report.Items = append(report.Items, item)
	}
	return report, rows.Err()
}

func scanProducts(rows *sql.Rows) ([]models.Product, error) {
	var products []models.Product
	for rows.Next() {
		var product models.Product
		var createdAt string
		var active int
		if err := rows.Scan(&product.ID, &product.Name, &product.Category, &product.CostPrice, &product.SellPrice, &product.Unit, &product.Stock, &active, &createdAt); err != nil {
			return nil, err
		}
		product.IsActive = active == 1
		product.CreatedAt = parseDBTime(createdAt)
		products = append(products, product)
	}
	return products, rows.Err()
}

func dayBounds(date time.Time) (time.Time, time.Time) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	return start, start.Add(24 * time.Hour)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatDBTime(value time.Time) string {
	return value.Format(sqliteDateTimeLayout)
}

func parseDBTime(raw string) time.Time {
	parsed, err := time.ParseInLocation(sqliteDateTimeLayout, raw, time.UTC)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed
	}
	return time.Time{}
}
