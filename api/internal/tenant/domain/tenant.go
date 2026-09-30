// Package domain (tenant): konsep tenant SaaS tanpa ketergantungan infrastruktur.
package domain

import "time"

type Status string

const (
	StatusTrial     Status = "trial"
	StatusActive    Status = "active"
	StatusPastDue   Status = "past_due"
	StatusSuspended Status = "suspended"
	StatusCancelled Status = "cancelled"
)

const (
	TrialPlanCode = "trial"
	// Kode outlet pertama; dipakai sebagai KODETOKO pada nomor struk (6.7) dan dapat diubah owner.
	DefaultStoreCode = "MAIN"
)

// TrialEndsAt menghitung akhir masa trial. Durasi berasal dari konfigurasi (Q17 masih terbuka).
func TrialEndsAt(now time.Time, days int) time.Time {
	return now.UTC().AddDate(0, 0, days)
}
