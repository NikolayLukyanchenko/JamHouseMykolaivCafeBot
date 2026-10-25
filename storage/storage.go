package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

const sqliteDateTimeLayout = "2006-01-02 15:04:05"

type Storage struct {
	db     *sql.DB
	remote bool
}

// New opens a local SQLite file.
func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Storage{db: db}, nil
}

// NewTurso connects to a Turso (libSQL) database, e.g.
// libsql://bot-jam-house-cafe-<org>.turso.io. The token may be empty only
// for a local libSQL server.
func NewTurso(url, authToken string) (*Storage, error) {
	var opts []libsql.Option
	if authToken != "" {
		opts = append(opts, libsql.WithAuthToken(authToken))
	}
	connector, err := libsql.NewConnector(url, opts...)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	return &Storage{db: db, remote: true}, nil
}

// Remote reports whether the database is Turso rather than a local file.
func (s *Storage) Remote() bool {
	return s.remote
}

// Ping checks that the database is reachable and the token is valid. It runs
// a real query because the libSQL driver's own ping is never called by
// database/sql and opening a connection does not touch the network.
func (s *Storage) Ping(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) Init(ctx context.Context) error {
	var statements []string
	if !s.remote {
		statements = append(statements, `PRAGMA foreign_keys = ON;`)
	}
	statements = append(statements,
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
		`CREATE TABLE IF NOT EXISTS voided_sales (
sale_id INTEGER PRIMARY KEY,
user_id INTEGER NOT NULL,
voided_by INTEGER NOT NULL,
total REAL NOT NULL,
payment_method TEXT NOT NULL,
items TEXT NOT NULL,
sold_at DATETIME NOT NULL,
voided_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`,
		`CREATE TABLE IF NOT EXISTS settings (
key TEXT PRIMARY KEY,
value TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS purchase_items (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL,
unit TEXT NOT NULL DEFAULT '',
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`,
	)
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

func (s *Storage) UpdateProductUnit(ctx context.Context, productID int64, unit string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET unit = ? WHERE id = ?`, unit, productID)
	return err
}

func (s *Storage) UpdateProductStock(ctx context.Context, productID int64, stock float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET stock = ? WHERE id = ?`, stock, productID)
	return err
}

// SetProductActive hides a product from the menu and sales (false) or
// returns it (true). Products are never hard-deleted because past sales and
// reports reference them.
func (s *Storage) SetProductActive(ctx context.Context, productID int64, active bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE products SET is_active = ? WHERE id = ?`, boolToInt(active), productID)
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
	items, err := s.soldItems(ctx, start, end, userID)
	if err != nil {
		return summary, err
	}
	summary.Items = items
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
COALESCE(SUM(cost_total), 0),
COUNT(*)
FROM sales
WHERE created_at >= ? AND created_at < ?
`, models.PaymentCash, models.PaymentCard, formatDBTime(start), formatDBTime(end))
	if err := row.Scan(&report.CashRevenue, &report.CardRevenue, &report.TotalRevenue, &report.CostTotal, &report.Checks); err != nil {
		return report, err
	}
	report.Profit = report.TotalRevenue - report.CostTotal

	items, err := s.soldItems(ctx, start, end, 0)
	if err != nil {
		return report, err
	}
	report.Items = items

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(total), 0) FROM voided_sales WHERE sold_at >= ? AND sold_at < ?`,
		formatDBTime(start), formatDBTime(end)).Scan(&report.VoidedChecks, &report.VoidedTotal); err != nil {
		return report, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT s.user_id, COALESCE(u.full_name, ''), COUNT(*),
COALESCE(SUM(CASE WHEN s.payment_method = ? THEN s.total END), 0),
COALESCE(SUM(CASE WHEN s.payment_method = ? THEN s.total END), 0),
COALESCE(SUM(s.total), 0)
FROM sales s
LEFT JOIN users u ON u.user_id = s.user_id
WHERE s.created_at >= ? AND s.created_at < ?
GROUP BY s.user_id
ORDER BY 6 DESC
`, models.PaymentCash, models.PaymentCard, formatDBTime(start), formatDBTime(end))
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var seller models.SellerSummary
		if err := rows.Scan(&seller.UserID, &seller.Name, &seller.Checks, &seller.Cash, &seller.Card, &seller.Total); err != nil {
			return report, err
		}
		report.Sellers = append(report.Sellers, seller)
	}
	return report, rows.Err()
}

// ListChecks returns the day's sales in chronological order with their
// items. userID 0 means all sellers.
func (s *Storage) ListChecks(ctx context.Context, date time.Time, userID int64) ([]models.SaleCheck, error) {
	start, end := dayBounds(date)
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, s.user_id, COALESCE(u.full_name, ''), s.total, s.payment_method, s.created_at,
COALESCE(si.name, ''), COALESCE(si.qty, 0)
FROM sales s
LEFT JOIN users u ON u.user_id = s.user_id
LEFT JOIN sale_items si ON si.sale_id = s.id
WHERE s.created_at >= ? AND s.created_at < ? AND (? = 0 OR s.user_id = ?)
ORDER BY s.created_at, s.id, si.id
`, formatDBTime(start), formatDBTime(end), userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []models.SaleCheck
	for rows.Next() {
		var check models.SaleCheck
		var createdAt, itemName string
		var qty float64
		if err := rows.Scan(&check.ID, &check.UserID, &check.SellerName, &check.Total, &check.PaymentMethod, &createdAt, &itemName, &qty); err != nil {
			return nil, err
		}
		if n := len(checks); n == 0 || checks[n-1].ID != check.ID {
			check.CreatedAt = parseDBTime(createdAt).In(time.Local)
			checks = append(checks, check)
		}
		if itemName != "" {
			last := &checks[len(checks)-1]
			last.Items = append(last.Items, models.SaleItem{SaleID: check.ID, Name: itemName, Qty: qty})
		}
	}
	return checks, rows.Err()
}

// TopProducts returns active products ordered by quantity sold since the
// given time, then by name, so the list is filled even with few sales.
func (s *Storage) TopProducts(ctx context.Context, since time.Time, limit int) ([]models.Product, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.id, p.name, p.category, p.cost_price, p.sell_price, p.unit, p.stock, p.is_active, p.created_at
FROM products p
LEFT JOIN (
	SELECT si.product_id AS product_id, SUM(si.qty) AS qty
	FROM sale_items si JOIN sales s ON s.id = si.sale_id
	WHERE s.created_at >= ?
	GROUP BY si.product_id
) sold ON sold.product_id = p.id
WHERE p.is_active = 1
ORDER BY COALESCE(sold.qty, 0) DESC, p.name
LIMIT ?
`, formatDBTime(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

var (
	ErrSaleNotFound  = errors.New("чек не знайдено або вже скасовано")
	ErrVoidForbidden = errors.New("скасувати чек може лише продавець, який його провів, або адміністратор")
	ErrVoidTooLate   = errors.New("час для скасування чеку минув")
)

// VoidSale cancels a sale: returns its items to stock, records it in
// voided_sales and removes it from sales, so reports no longer count it.
// Unless asAdmin, only the seller may void, and only within window.
func (s *Storage) VoidSale(ctx context.Context, saleID, userID int64, asAdmin bool, window time.Duration, now time.Time) (models.Sale, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Sale{}, err
	}
	defer tx.Rollback()

	var sale models.Sale
	var createdAt string
	err = tx.QueryRowContext(ctx, `SELECT id, user_id, total, cost_total, payment_method, created_at FROM sales WHERE id = ?`, saleID).
		Scan(&sale.ID, &sale.UserID, &sale.Total, &sale.CostTotal, &sale.PaymentMethod, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Sale{}, ErrSaleNotFound
	}
	if err != nil {
		return models.Sale{}, err
	}
	sale.CreatedAt = parseDBTime(createdAt)
	if !asAdmin {
		if sale.UserID != userID {
			return models.Sale{}, ErrVoidForbidden
		}
		if now.Sub(sale.CreatedAt) > window {
			return models.Sale{}, ErrVoidTooLate
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT product_id, name, qty FROM sale_items WHERE sale_id = ? ORDER BY id`, saleID)
	if err != nil {
		return models.Sale{}, err
	}
	type line struct {
		productID int64
		name      string
		qty       float64
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.productID, &l.name, &l.qty); err != nil {
			rows.Close()
			return models.Sale{}, err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return models.Sale{}, err
	}

	names := make([]string, 0, len(lines))
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, `UPDATE products SET stock = stock + ? WHERE id = ?`, l.qty, l.productID); err != nil {
			return models.Sale{}, err
		}
		names = append(names, fmt.Sprintf("%s ×%s", l.name, strconv.FormatFloat(l.qty, 'f', -1, 64)))
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO voided_sales (sale_id, user_id, voided_by, total, payment_method, items, sold_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sale.ID, sale.UserID, userID, sale.Total, sale.PaymentMethod, strings.Join(names, ", "), formatDBTime(sale.CreatedAt)); err != nil {
		return models.Sale{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sale_items WHERE sale_id = ?`, saleID); err != nil {
		return models.Sale{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sales WHERE id = ?`, saleID); err != nil {
		return models.Sale{}, err
	}
	return sale, tx.Commit()
}

// soldItems aggregates sold quantities, revenue and cost per product in
// [start, end). userID 0 means all sellers. Revenue and cost use the prices
// saved with each sale, so later price changes do not rewrite history.
func (s *Storage) soldItems(ctx context.Context, start, end time.Time, userID int64) ([]models.DailyReportItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT si.product_id, si.name, COALESCE(p.category, ''), COALESCE(p.unit, ''),
SUM(si.qty), SUM(si.qty * si.sell_price), SUM(si.qty * si.cost_price)
FROM sale_items si
JOIN sales s ON s.id = si.sale_id
LEFT JOIN products p ON p.id = si.product_id
WHERE s.created_at >= ? AND s.created_at < ? AND (? = 0 OR s.user_id = ?)
GROUP BY si.product_id, si.name
ORDER BY 6 DESC, si.name
`, formatDBTime(start), formatDBTime(end), userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.DailyReportItem
	for rows.Next() {
		var item models.DailyReportItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Category, &item.Unit, &item.Qty, &item.Revenue, &item.Cost); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Storage) CreatePurchaseItem(ctx context.Context, name, unit string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO purchase_items (name, unit) VALUES (?, ?)`, name, unit)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Storage) GetPurchaseItem(ctx context.Context, itemID int64) (models.PurchaseItem, error) {
	var item models.PurchaseItem
	var createdAt string
	row := s.db.QueryRowContext(ctx, `SELECT id, name, unit, created_at FROM purchase_items WHERE id = ?`, itemID)
	if err := row.Scan(&item.ID, &item.Name, &item.Unit, &createdAt); err != nil {
		return models.PurchaseItem{}, err
	}
	item.CreatedAt = parseDBTime(createdAt)
	return item, nil
}

func (s *Storage) ListPurchaseItems(ctx context.Context) ([]models.PurchaseItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, unit, created_at FROM purchase_items ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.PurchaseItem
	for rows.Next() {
		var item models.PurchaseItem
		var createdAt string
		if err := rows.Scan(&item.ID, &item.Name, &item.Unit, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = parseDBTime(createdAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Storage) UpdatePurchaseItem(ctx context.Context, itemID int64, name, unit string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE purchase_items SET name = ?, unit = ? WHERE id = ?`, name, unit, itemID)
	return err
}

func (s *Storage) DeletePurchaseItem(ctx context.Context, itemID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM purchase_items WHERE id = ?`, itemID)
	return err
}

// Stats counts the main records; used to verify a migration.
type Stats struct {
	Users, Products, Sales, SaleItems, PurchaseItems int
}

func (s *Storage) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
(SELECT COUNT(*) FROM users),
(SELECT COUNT(*) FROM products),
(SELECT COUNT(*) FROM sales),
(SELECT COUNT(*) FROM sale_items),
(SELECT COUNT(*) FROM purchase_items)`).Scan(&st.Users, &st.Products, &st.Sales, &st.SaleItems, &st.PurchaseItems)
	return st, err
}

func (st Stats) String() string {
	return fmt.Sprintf("users=%d products=%d sales=%d sale_items=%d purchase_items=%d", st.Users, st.Products, st.Sales, st.SaleItems, st.PurchaseItems)
}

// Backup writes a consistent copy of the local SQLite database to dest (which
// must not exist) in WAL mode, ready for `turso db import`.
func (s *Storage) Backup(ctx context.Context, dest string) error {
	if s.remote {
		return errors.New("резервна копія файлом доступна лише для локальної SQLite-бази")
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		return err
	}
	copyDB, err := sql.Open("sqlite", dest)
	if err != nil {
		return err
	}
	defer copyDB.Close()
	var mode string
	if err := copyDB.QueryRowContext(ctx, `PRAGMA journal_mode = WAL`).Scan(&mode); err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("не вдалося перевести копію в режим WAL: %s", mode)
	}
	_, err = copyDB.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// SalesDays returns the local calendar days (as "2006-01-02") in [from, to)
// that have at least one sale.
func (s *Storage) SalesDays(ctx context.Context, from, to time.Time) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT created_at FROM sales WHERE created_at >= ? AND created_at < ?`, formatDBTime(from), formatDBTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	days := make(map[string]bool)
	for rows.Next() {
		var createdAt string
		if err := rows.Scan(&createdAt); err != nil {
			return nil, err
		}
		if t := parseDBTime(createdAt); !t.IsZero() {
			days[t.In(time.Local).Format("2006-01-02")] = true
		}
	}
	return days, rows.Err()
}

// GetSetting returns "" when the key is not set.
func (s *Storage) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Storage) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *Storage) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
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

// formatDBTime converts to UTC because created_at columns are filled by
// SQLite's CURRENT_TIMESTAMP, which is always UTC.
func formatDBTime(value time.Time) string {
	return value.UTC().Format(sqliteDateTimeLayout)
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
