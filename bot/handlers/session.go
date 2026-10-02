package handlers

import (
	"sync"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

const (
	stateNone               = ""
	stateAwaitOrderQty      = "await_order_qty"
	stateAwaitPurchaseNote  = "await_purchase_note"
	stateAwaitPItemsAdd     = "await_purchase_items_add"
	stateAwaitPItemEdit     = "await_purchase_item_edit"
	stateAwaitProductName   = "await_product_name"
	stateAwaitProductCost   = "await_product_cost"
	stateAwaitProductSell   = "await_product_sell"
	stateAwaitProductUnit   = "await_product_unit"
	stateAwaitProductStock  = "await_product_stock"
	stateAwaitEditName      = "await_edit_name"
	stateAwaitEditCost      = "await_edit_cost"
	stateAwaitEditSell      = "await_edit_sell"
	stateAwaitSetStock      = "await_set_stock"
	stateAwaitIncreaseStock = "await_increase_stock"
)

type productDraft struct {
	Name      string
	Category  string
	CostPrice float64
	SellPrice float64
	Unit      string
	Stock     float64
}

type session struct {
	State             string
	SelectedProductID int64
	EditingProductID  int64
	PaymentMethod     string
	DraftProduct      productDraft
	Cart              []models.OrderItem

	EditingPurchaseItemID int64
	Purchase              purchaseDraft
}

// purchaseDraft is the purchase request being composed by tapping items.
type purchaseDraft struct {
	Qty  map[int64]int
	Note string
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[int64]*session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[int64]*session)}
}

func (m *SessionManager) get(userID int64) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[userID]; ok {
		return s
	}
	s := &session{}
	m.sessions[userID] = s
	return s
}

func (m *SessionManager) SetState(userID int64, state string) {
	m.get(userID).State = state
}

func (m *SessionManager) ClearState(userID int64) {
	s := m.get(userID)
	s.State = stateNone
	s.SelectedProductID = 0
	s.PaymentMethod = ""
}

func (m *SessionManager) ResetDraft(userID int64) {
	m.get(userID).DraftProduct = productDraft{}
}

func (m *SessionManager) AddToCart(userID int64, item models.OrderItem) {
	s := m.get(userID)
	for i := range s.Cart {
		if s.Cart[i].ProductID == item.ProductID {
			s.Cart[i].Qty += item.Qty
			return
		}
	}
	s.Cart = append(s.Cart, item)
}

func (m *SessionManager) Cart(userID int64) []models.OrderItem {
	s := m.get(userID)
	cart := make([]models.OrderItem, len(s.Cart))
	copy(cart, s.Cart)
	return cart
}

func (m *SessionManager) ClearCart(userID int64) {
	s := m.get(userID)
	s.Cart = nil
	s.PaymentMethod = ""
}

func (m *SessionManager) ResetPurchase(userID int64) {
	m.get(userID).Purchase = purchaseDraft{Qty: make(map[int64]int)}
}

// AdjustPurchaseQty changes the quantity of a purchase item by delta and
// drops it from the draft when it reaches zero.
func (m *SessionManager) AdjustPurchaseQty(userID, itemID int64, delta int) {
	s := m.get(userID)
	if s.Purchase.Qty == nil {
		s.Purchase.Qty = make(map[int64]int)
	}
	n := s.Purchase.Qty[itemID] + delta
	if n <= 0 {
		delete(s.Purchase.Qty, itemID)
		return
	}
	s.Purchase.Qty[itemID] = n
}
