package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// days returns n consecutive dates ending at `end`
func days(end string, n int) []string {
	t, _ := time.Parse(dateLayout, end)
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[n-1-i] = t.AddDate(0, 0, -i).Format(dateLayout)
	}
	return out
}

func TestComputeStatus(t *testing.T) {
	const today = "2026-11-10"

	tests := []struct {
		name        string
		checkIns    []string
		wantCurrent int
		wantLongest int
		wantStage   PlantStage
		wantHealth  Health
	}{
		{"no check-ins", nil, 0, 0, StageSeed, Healthy},
		{"checked in today only", []string{"2026-11-10"}, 1, 1, StageSprout, Healthy},
		{"checked in yesterday only (thirsty)", []string{"2026-11-09"}, 1, 1, StageSprout, Thirsty},
		{"last check-in 2 days ago (wilted)", []string{"2026-11-08"}, 0, 1, StageSeed, Wilted},
		{"3 day streak", days("2026-11-10", 3), 3, 3, StageYoung, Healthy},
		{"2 day streak is still sprout", days("2026-11-10", 2), 2, 2, StageSprout, Healthy},
		{"7 day streak", days("2026-11-10", 7), 7, 7, StageLeafy, Healthy},
		{"20 day streak", days("2026-11-10", 20), 20, 20, StageLeafy, Healthy},
		{"21 day streak", days("2026-11-10", 21), 21, 21, StageBlooming, Healthy},
		{"gap in the middle", []string{"2026-11-03", "2026-11-04", "2026-11-05", "2026-11-08", "2026-11-09"}, 2, 3, StageSprout, Thirsty},
		{"duplicates are ignored", []string{"2026-11-10", "2026-11-10"}, 1, 1, StageSprout, Healthy},
		{"unsorted input", []string{"2026-11-10", "2026-11-08", "2026-11-09"}, 3, 3, StageYoung, Healthy},
		{"wilted keeps longest streak", days("2026-11-01", 5), 0, 5, StageSeed, Wilted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeStatus(tc.checkIns, today)
			if got.CurrentStreak != tc.wantCurrent {
				t.Errorf("current streak = %d, want %d", got.CurrentStreak, tc.wantCurrent)
			}
			if got.LongestStreak != tc.wantLongest {
				t.Errorf("longest streak = %d, want %d", got.LongestStreak, tc.wantLongest)
			}
			if got.Stage != tc.wantStage {
				t.Errorf("stage = %s, want %s", got.Stage, tc.wantStage)
			}
			if got.Health != tc.wantHealth {
				t.Errorf("health = %s, want %s", got.Health, tc.wantHealth)
			}
		})
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := NewJSONRepo(filepath.Join(t.TempDir(), "habits.json"))
	if err != nil {
		t.Fatal(err)
	}
	return (&server{repo: repo}).routes()
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPIFlow(t *testing.T) {
	h := newTestHandler(t)

	if rec := do(h, "POST", "/api/habits", `{"name":""}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name: got %d, want 400", rec.Code)
	}

	rec := do(h, "POST", "/api/habits", `{"name":"Read 20 pages","plantType":"fern"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rec.Code)
	}
	var created HabitView
	json.Unmarshal(rec.Body.Bytes(), &created)
	base := "/api/habits/" + created.ID

	steps := []struct {
		name, method, path, body string
		want                     int
	}{
		{"check in", "POST", base + "/checkin", "", 200},
		{"check in twice", "POST", base + "/checkin", "", 409},
		{"undo check-in", "DELETE", base + "/checkin", "", 200},
		{"undo again", "DELETE", base + "/checkin", "", 409},
		{"get one", "GET", base, "", 200},
		{"edit", "PUT", base, `{"name":"Read 30 pages","plantType":"rose"}`, 200},
		{"edit with bad plant", "PUT", base, `{"plantType":"banana"}`, 400},
		{"garden summary", "GET", "/api/garden", "", 200},
		{"delete", "DELETE", base, "", 204},
		{"get deleted", "GET", base, "", 404},
	}
	for _, s := range steps {
		if rec := do(h, s.method, s.path, s.body); rec.Code != s.want {
			t.Errorf("%s: got %d, want %d", s.name, rec.Code, s.want)
		}
	}
}

func TestConcurrentRequests(t *testing.T) {
	h := newTestHandler(t)

	// 50 habits created at the same time
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := do(h, "POST", "/api/habits", `{"name":"Habit"}`); rec.Code != http.StatusCreated {
				t.Errorf("create: got %d, want 201", rec.Code)
			}
		}()
	}
	wg.Wait()

	rec := do(h, "GET", "/api/habits", "")
	var list []HabitView
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 50 {
		t.Fatalf("got %d habits, want 50", len(list))
	}

	// 50 check-ins on the same habit at the same time: only one may succeed
	id := list[0].ID
	var mu sync.Mutex
	success := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := do(h, "POST", "/api/habits/"+id+"/checkin", ""); rec.Code == http.StatusOK {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("%d check-ins succeeded, want exactly 1", success)
	}
}
