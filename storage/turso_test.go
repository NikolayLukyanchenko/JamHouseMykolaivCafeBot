package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

// fakeHrana is a minimal Hrana v2 (Turso HTTP protocol) server backed by a
// local SQLite file. It lets the real libSQL driver run our queries end to
// end, including streams (batons) that carry transactions.
type fakeHrana struct {
	t       *testing.T
	db      *sql.DB
	token   string
	mu      sync.Mutex
	streams map[string]*sql.Conn
	next    int
}

type hranaValue struct {
	Type   string `json:"type"`
	Value  any    `json:"value,omitempty"`
	Base64 string `json:"base64,omitempty"`
}

type hranaStmt struct {
	SQL      string       `json:"sql"`
	Args     []hranaValue `json:"args"`
	WantRows bool         `json:"want_rows"`
}

func (f *fakeHrana) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v2/pipeline" {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+f.token {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"Unauthorized"}`)
		return
	}
	var req struct {
		Baton    string `json:"baton"`
		Requests []struct {
			Type string     `json:"type"`
			Stmt *hranaStmt `json:"stmt"`
		} `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	conn := f.streams[req.Baton]
	delete(f.streams, req.Baton)
	if conn == nil {
		if req.Baton != "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":"stream expired","code":"STREAM_EXPIRED"}`)
			return
		}
		var err error
		if conn, err = f.db.Conn(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	results := make([]any, 0, len(req.Requests))
	closed := false
	for _, sr := range req.Requests {
		switch sr.Type {
		case "close":
			conn.Close()
			closed = true
			results = append(results, map[string]any{"type": "ok", "response": map[string]any{"type": "close"}})
		case "execute":
			res, err := f.execute(r.Context(), conn, sr.Stmt)
			if err != nil {
				results = append(results, map[string]any{"type": "error", "error": map[string]any{"message": err.Error()}})
				continue
			}
			results = append(results, map[string]any{"type": "ok", "response": map[string]any{"type": "execute", "result": res}})
		default:
			f.t.Errorf("fake Turso: unsupported request type %q", sr.Type)
			results = append(results, map[string]any{"type": "error", "error": map[string]any{"message": "unsupported"}})
		}
	}
	baton := ""
	if !closed {
		f.next++
		baton = "b" + strconv.Itoa(f.next)
		f.streams[baton] = conn
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"baton": baton, "results": results})
}

func (f *fakeHrana) execute(ctx context.Context, conn *sql.Conn, stmt *hranaStmt) (map[string]any, error) {
	args := make([]any, len(stmt.Args))
	for i, a := range stmt.Args {
		switch a.Type {
		case "integer":
			n, err := strconv.ParseInt(a.Value.(string), 10, 64)
			if err != nil {
				return nil, err
			}
			args[i] = n
		case "float":
			args[i] = a.Value.(float64)
		case "text":
			args[i] = a.Value.(string)
		case "null":
			args[i] = nil
		default:
			return nil, fmt.Errorf("unsupported arg type %q", a.Type)
		}
	}
	if !stmt.WantRows {
		res, err := conn.ExecContext(ctx, stmt.SQL, args...)
		if err != nil {
			return nil, err
		}
		affected, _ := res.RowsAffected()
		lastID, _ := res.LastInsertId()
		return map[string]any{"cols": []any{}, "rows": []any{}, "affected_row_count": affected, "last_insert_rowid": strconv.FormatInt(lastID, 10)}, nil
	}
	rows, err := conn.QueryContext(ctx, stmt.SQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	cols := make([]map[string]any, len(types))
	for i, ct := range types {
		cols[i] = map[string]any{"name": ct.Name(), "decltype": ct.DatabaseTypeName()}
	}
	outRows := [][]hranaValue{}
	for rows.Next() {
		raw := make([]any, len(types))
		ptrs := make([]any, len(types))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]hranaValue, len(raw))
		for i, v := range raw {
			switch v := v.(type) {
			case nil:
				row[i] = hranaValue{Type: "null"}
			case int64:
				row[i] = hranaValue{Type: "integer", Value: strconv.FormatInt(v, 10)}
			case float64:
				row[i] = hranaValue{Type: "float", Value: v}
			case string:
				row[i] = hranaValue{Type: "text", Value: v}
			case time.Time:
				// Turso returns DATETIME columns as the stored text.
				row[i] = hranaValue{Type: "text", Value: v.UTC().Format(sqliteDateTimeLayout)}
			default:
				return nil, fmt.Errorf("unsupported column value %T", v)
			}
		}
		outRows = append(outRows, row)
	}
	return map[string]any{"cols": cols, "rows": outRows, "affected_row_count": 0}, rows.Err()
}

func newFakeTurso(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "turso.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	srv := httptest.NewServer(&fakeHrana{t: t, db: db, token: "secret", streams: map[string]*sql.Conn{}})
	t.Cleanup(srv.Close)
	return srv
}

// TestTursoEndToEnd runs the bot's storage through the real libSQL driver.
func TestTursoEndToEnd(t *testing.T) {
	srv := newFakeTurso(t)
	ctx := context.Background()

	bad, err := NewTurso(srv.URL, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.Ping(ctx); err == nil {
		t.Fatal("a wrong token must fail on Ping")
	}
	bad.Close()

	store, err := NewTurso(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if !store.Remote() {
		t.Fatal("expected remote storage")
	}
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init must be idempotent: %v", err)
	}
	if err := store.SeedAdmins(ctx, []int64{1, 2}); err != nil {
		t.Fatalf("SeedAdmins: %v", err)
	}
	user, err := store.GetUser(ctx, 1)
	if err != nil || user.Role != models.RoleAdmin || user.CreatedAt.IsZero() {
		t.Fatalf("GetUser: %+v %v", user, err)
	}

	latte, err := store.CreateProduct(ctx, models.Product{Name: "Лате", Category: models.CategoryDrinks, CostPrice: 30, SellPrice: 70.5, Unit: "чашка", Stock: 10, IsActive: true})
	if err != nil || latte == 0 {
		t.Fatalf("CreateProduct: %d %v", latte, err)
	}
	if err := store.UpdateProductUnit(ctx, latte, "склянка"); err != nil {
		t.Fatal(err)
	}

	sale, items, err := store.RecordSale(ctx, 1, models.PaymentCard, []models.OrderItem{{ProductID: latte, Qty: 2}})
	if err != nil {
		t.Fatalf("RecordSale: %v", err)
	}
	if sale.ID == 0 || sale.Total != 141 || len(items) != 1 || items[0].ID == 0 {
		t.Fatalf("unexpected sale: %+v %+v", sale, items)
	}
	if _, _, err := store.RecordSale(ctx, 2, models.PaymentCash, []models.OrderItem{{ProductID: latte, Qty: 100}}); err == nil {
		t.Fatal("overselling must fail")
	}
	product, err := store.GetProduct(ctx, latte)
	if err != nil || product.Stock != 8 || product.Unit != "склянка" || !product.IsActive {
		t.Fatalf("failed sale must be rolled back: %+v %v", product, err)
	}

	now := time.Now()
	report, err := store.GetDailyReport(ctx, now)
	if err != nil {
		t.Fatalf("GetDailyReport: %v", err)
	}
	if report.Checks != 1 || report.TotalRevenue != 141 || report.CardRevenue != 141 || report.CostTotal != 60 ||
		len(report.Items) != 1 || report.Items[0].Unit != "склянка" || len(report.Sellers) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	summary, err := store.GetUserSalesSummary(ctx, 1, now)
	if err != nil || summary.Checks != 1 || summary.GrandTotal != 141 {
		t.Fatalf("GetUserSalesSummary: %+v %v", summary, err)
	}
	checks, err := store.ListChecks(ctx, now, 0)
	if err != nil || len(checks) != 1 || checks[0].CreatedAt.IsZero() || len(checks[0].Items) != 1 {
		t.Fatalf("ListChecks: %+v %v", checks, err)
	}
	days, err := store.SalesDays(ctx, now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))
	if err != nil || !days[now.Format("2006-01-02")] {
		t.Fatalf("SalesDays: %v %v", days, err)
	}

	if err := store.SetSetting(ctx, "purchase_chat_id", "-100"); err != nil {
		t.Fatal(err)
	}
	if v, err := store.GetSetting(ctx, "purchase_chat_id"); err != nil || v != "-100" {
		t.Fatalf("GetSetting: %q %v", v, err)
	}
	if v, err := store.GetSetting(ctx, "missing"); err != nil || v != "" {
		t.Fatalf("GetSetting(missing): %q %v", v, err)
	}
	if _, err := store.CreatePurchaseItem(ctx, "Молоко", "л"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetProductActive(ctx, latte, false); err != nil {
		t.Fatal(err)
	}
	if active, err := store.ListProducts(ctx, false); err != nil || len(active) != 0 {
		t.Fatalf("hidden product listed: %+v %v", active, err)
	}

	top, err := store.TopProducts(ctx, now.AddDate(0, 0, -14), 9)
	if err != nil || len(top) != 0 {
		t.Fatalf("TopProducts (only hidden product): %+v %v", top, err)
	}
	if err := store.SetProductActive(ctx, latte, true); err != nil {
		t.Fatal(err)
	}
	extra, _, err := store.RecordSale(ctx, 2, models.PaymentCash, []models.OrderItem{{ProductID: latte, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.VoidSale(ctx, extra.ID, 2, false, 3*time.Minute, time.Now()); err != nil {
		t.Fatalf("VoidSale: %v", err)
	}
	if p, _ := store.GetProduct(ctx, latte); p.Stock != 8 {
		t.Fatalf("void must return stock: %v", p.Stock)
	}
	if err := store.SetProductActive(ctx, latte, false); err != nil {
		t.Fatal(err)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats != (Stats{Users: 2, Products: 1, Sales: 1, SaleItems: 1, PurchaseItems: 1}) {
		t.Fatalf("unexpected stats: %v", stats)
	}
	if err := store.Backup(ctx, filepath.Join(t.TempDir(), "x.db")); err == nil {
		t.Fatal("file backup must be refused for Turso")
	}
}

// TestTursoAfterStreamsExpire checks that requests keep working after the
// server drops all streams, as Turso does after ~10s of inactivity. The
// driver closes its stream whenever a connection returns to the pool, so no
// expired stream is reused.
func TestTursoAfterStreamsExpire(t *testing.T) {
	srv := newFakeTurso(t)
	ctx := context.Background()
	store, err := NewTurso(srv.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	h := srv.Config.Handler.(*fakeHrana)
	h.mu.Lock()
	for baton, conn := range h.streams {
		conn.Close()
		delete(h.streams, baton)
	}
	h.mu.Unlock()
	if _, err := store.Stats(ctx); err != nil {
		t.Fatalf("request after streams expired failed: %v", err)
	}
}
