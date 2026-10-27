package store_test

import (
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

var now = time.Date(2026, 10, 26, 12, 30, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func newGroup(id string, version int64) store.CreateLedgerParams {
	return store.CreateLedgerParams{
		ID:            id,
		Kind:          "group",
		Name:          "Viaje a Cartagena",
		Currency:      ptr("COP"),
		SimplifyDebts: true,
		Version:       version,
		CreatedAt:     now,
	}
}
