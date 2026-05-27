package ai

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

func TestFirstRunObservationStartUsesLatestKlineOnly(t *testing.T) {
	klines := makeTestWatchKlines(12)

	got := firstRunObservationStart(klines)
	want := len(klines) - 1
	if got != want {
		t.Fatalf("firstRunObservationStart() = %d, want %d", got, want)
	}
}

func TestFindNewKlinesDefaultsToLatestWhenNoResult(t *testing.T) {
	klines := makeTestWatchKlines(5)

	got := (&WatchEngine{}).findNewKlines(klines, nil)
	if len(got) != 1 {
		t.Fatalf("findNewKlines() returned %d klines, want 1", len(got))
	}
	if !got[0].OpenTime.Equal(klines[len(klines)-1].OpenTime) {
		t.Fatalf("findNewKlines() returned %v, want latest %v", got[0].OpenTime, klines[len(klines)-1].OpenTime)
	}
}

func TestFindNewKlinesUsesPersistedLatestStepTime(t *testing.T) {
	klines := makeTestWatchKlines(5)
	result, _ := json.Marshal(map[string]interface{}{
		"steps": []map[string]interface{}{
			{"kline_time": klines[1].OpenTime.UnixMilli()},
			{"kline_time": klines[2].OpenTime.UnixMilli()},
		},
	})

	got := (&WatchEngine{}).findNewKlines(klines, result)
	if len(got) != 2 {
		t.Fatalf("findNewKlines() returned %d klines, want 2", len(got))
	}
	if !got[0].OpenTime.Equal(klines[3].OpenTime) || !got[1].OpenTime.Equal(klines[4].OpenTime) {
		t.Fatalf("findNewKlines() returned wrong range: %+v", got)
	}
}

func makeTestWatchKlines(n int) []models.Kline {
	base := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	klines := make([]models.Kline, n)
	for i := 0; i < n; i++ {
		klines[i] = models.Kline{
			OpenTime:   base.Add(time.Duration(i) * time.Hour),
			ClosePrice: float64(100 + i),
		}
	}
	return klines
}
