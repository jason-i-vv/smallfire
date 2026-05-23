package trading

import (
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

func TestFilterKlinesAfterEntry(t *testing.T) {
	entry := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)

	t.Run("new position skips entry kline", func(t *testing.T) {
		klines := []models.Kline{
			{OpenTime: entry.Add(-15 * time.Minute)},
			{OpenTime: entry},
			{OpenTime: entry.Add(15 * time.Minute)},
			{OpenTime: entry.Add(30 * time.Minute)},
		}

		got := filterKlinesAfterEntry(klines, entry)
		if len(got) != 2 {
			t.Fatalf("expected 2 klines after entry, got %d", len(got))
		}
		if !got[0].OpenTime.Equal(entry.Add(15 * time.Minute)) {
			t.Fatalf("expected first checked kline after entry, got %s", got[0].OpenTime)
		}
	})

	t.Run("old position scans recent window", func(t *testing.T) {
		klines := []models.Kline{
			{OpenTime: entry.Add(7 * 24 * time.Hour)},
			{OpenTime: entry.Add(7*24*time.Hour + 15*time.Minute)},
		}

		got := filterKlinesAfterEntry(klines, entry)
		if len(got) != len(klines) {
			t.Fatalf("expected all recent klines to be checked, got %d", len(got))
		}
	})

	t.Run("no future kline returns nil", func(t *testing.T) {
		klines := []models.Kline{
			{OpenTime: entry.Add(-15 * time.Minute)},
			{OpenTime: entry},
		}

		if got := filterKlinesAfterEntry(klines, entry); got != nil {
			t.Fatalf("expected nil when no kline is after entry, got %d", len(got))
		}
	})
}
