package handlers

import (
	"sync"

	"github.com/NikolayLukyanchenko/JamHouseMykolaivCafeBot/models"
)

const (
	stateNone               = ""
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
	stateAwaitEditUnit      = "await_edit_unit"
	stateAwaitBundleName    = "await_bundle_name"
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
	State            string
	EditingProductID int64
	DraftProduct     productDraft
	Cart             []models.OrderItem

	PosTab                string // active tab of the checkout screen
	Bundle                bundleDraft
	EditingBundleID       int64
	EditingPurchaseItemID int64
	Purchase              purchaseDraft
}

// bundleDraft is the bundle an admin is composing; ID 0 means a new one.
type bundleDraft struct {
	ID    int64
	Qty   map[int64]int
	Order []int64 // components in the order they were added
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

// AdjustCart changes an item's quantity by delta and drops it at zero.
func (m *SessionManager) AdjustCart(userID, productID int64, delta float64) {
	s := m.get(userID)
	for i := range s.Cart {
		if s.Cart[i].ProductID != productID {
			continue
		}
		s.Cart[i].Qty += delta
		if s.Cart[i].Qty <= 0 {
			s.Cart = append(s.Cart[:i], s.Cart[i+1:]...)
		}
		return
	}
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
