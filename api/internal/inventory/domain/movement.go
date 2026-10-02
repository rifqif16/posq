package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type MovementType string

const (
	TypePurchaseReceive MovementType = "purchase_receive"
	TypeWaste           MovementType = "waste"
	TypeAdjustment      MovementType = "adjustment"
	TypeOpname          MovementType = "opname"
)

const (
	MaxReasonLen  = 200
	MaxUnitCost   = int64(1_000_000_000)
	MaxCountItems = 200
	DefaultOpname = "Stok opname"
)

func KnownType(t string) bool {
	switch MovementType(t) {
	case TypePurchaseReceive, TypeWaste, TypeAdjustment, TypeOpname:
		return true
	}
	return false
}

type Issue struct {
	Field   string
	Message string
}

type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string { return "validasi gagal" }

type Movement struct {
	ID          uuid.UUID
	VariantID   uuid.UUID
	ProductName string
	VariantName string
	SKU         string
	Type        MovementType
	QtyDelta    int64
	UnitCost    *int64
	Reason      string
	UserID      uuid.UUID
	UserName    string
	CreatedAt   time.Time
}

type Level struct {
	VariantID   uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	VariantName string
	SKU         string
	QtyOnHand   int64
}

type MovementInput struct {
	StoreID   uuid.UUID
	VariantID uuid.UUID
	Type      MovementType
	QtyDelta  string
	UnitCost  *int64
	Reason    string
}

type ValidatedMovement struct {
	StoreID   uuid.UUID
	VariantID uuid.UUID
	Type      MovementType
	QtyDelta  int64
	UnitCost  *int64
	Reason    string
}

func (in MovementInput) Validate() (ValidatedMovement, error) {
	var issues []Issue
	if in.StoreID == uuid.Nil {
		issues = append(issues, Issue{"store_id", "Outlet wajib diisi"})
	}
	if in.VariantID == uuid.Nil {
		issues = append(issues, Issue{"variant_id", "Varian wajib diisi"})
	}
	switch in.Type {
	case TypePurchaseReceive, TypeWaste, TypeAdjustment:
	default:
		issues = append(issues, Issue{"type", "Tipe harus purchase_receive, waste, atau adjustment"})
	}

	qty, err := ParseQty(in.QtyDelta)
	switch {
	case err != nil || qty > MaxQtyAbs || qty < -MaxQtyAbs:
		issues = append(issues, Issue{"qty_delta", "Jumlah harus angka dengan maksimal 3 desimal"})
	case qty == 0:
		issues = append(issues, Issue{"qty_delta", "Jumlah tidak boleh nol"})
	case in.Type == TypePurchaseReceive && qty < 0:
		issues = append(issues, Issue{"qty_delta", "Stok masuk harus bernilai positif"})
	case in.Type == TypeWaste && qty > 0:
		issues = append(issues, Issue{"qty_delta", "Stok rusak/keluar harus bernilai negatif"})
	}

	reason := strings.TrimSpace(in.Reason)
	switch {
	case utf8.RuneCountInString(reason) > MaxReasonLen:
		issues = append(issues, Issue{"reason", "Alasan maksimal 200 karakter"})
	case reason == "" && (in.Type == TypeWaste || in.Type == TypeAdjustment):
		issues = append(issues, Issue{"reason", "Alasan wajib diisi"})
	}

	if in.UnitCost != nil {
		switch {
		case in.Type != TypePurchaseReceive:
			issues = append(issues, Issue{"unit_cost", "Harga satuan hanya untuk stok masuk"})
		case *in.UnitCost < 0 || *in.UnitCost > MaxUnitCost:
			issues = append(issues, Issue{"unit_cost", "Harga satuan harus antara 0 dan 1.000.000.000"})
		}
	}
	if len(issues) > 0 {
		return ValidatedMovement{}, &ValidationError{Issues: issues}
	}
	return ValidatedMovement{
		StoreID: in.StoreID, VariantID: in.VariantID, Type: in.Type, QtyDelta: qty, UnitCost: in.UnitCost, Reason: reason,
	}, nil
}

type CountItemInput struct {
	VariantID  uuid.UUID
	CountedQty string
}

type CountInput struct {
	StoreID uuid.UUID
	Reason  string
	Items   []CountItemInput
}

type CountItem struct {
	VariantID uuid.UUID
	Counted   int64
}

type ValidatedCount struct {
	StoreID uuid.UUID
	Reason  string
	Items   []CountItem
}

func (in CountInput) Validate() (ValidatedCount, error) {
	var issues []Issue
	if in.StoreID == uuid.Nil {
		issues = append(issues, Issue{"store_id", "Outlet wajib diisi"})
	}
	reason := strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(reason) > MaxReasonLen {
		issues = append(issues, Issue{"reason", "Alasan maksimal 200 karakter"})
	}
	if reason == "" {
		reason = DefaultOpname
	}
	if n := len(in.Items); n < 1 || n > MaxCountItems {
		issues = append(issues, Issue{"items", "Hasil hitung harus berisi 1 sampai 200 varian"})
		return ValidatedCount{}, &ValidationError{Issues: issues}
	}

	items := make([]CountItem, len(in.Items))
	seen := make(map[uuid.UUID]bool, len(in.Items))
	for i, it := range in.Items {
		path := "items[" + strconv.Itoa(i) + "]"
		if it.VariantID == uuid.Nil {
			issues = append(issues, Issue{path + ".variant_id", "Varian wajib diisi"})
		} else if seen[it.VariantID] {
			issues = append(issues, Issue{path + ".variant_id", "Varian dikirim lebih dari sekali"})
		}
		seen[it.VariantID] = true
		qty, err := ParseQty(it.CountedQty)
		if err != nil || qty < 0 || qty > MaxQtyAbs {
			issues = append(issues, Issue{path + ".counted_qty", "Jumlah hitung harus angka nol atau lebih dengan maksimal 3 desimal"})
		}
		items[i] = CountItem{VariantID: it.VariantID, Counted: qty}
	}
	if len(issues) > 0 {
		return ValidatedCount{}, &ValidationError{Issues: issues}
	}
	return ValidatedCount{StoreID: in.StoreID, Reason: reason, Items: items}, nil
}
