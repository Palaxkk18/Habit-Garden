package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

var (
	errAlreadyChecked = errors.New("already checked in today")
	errNotChecked     = errors.New("no check-in for today to undo")
)

var plantTypes = map[string]bool{"sunflower": true, "fern": true, "cactus": true, "rose": true, "tulip": true}

type habitInput struct {
	Name      string `json:"name"`
	PlantType string `json:"plantType"`
}

// HabitView is what the API sends: the stored habit + computed streak/plant state
type HabitView struct {
	Habit
	Status
}

type Summary struct {
	TotalHabits int `json:"totalHabits"`
	DoneToday   int `json:"doneToday"`
	Healthy     int `json:"healthy"`
	Thirsty     int `json:"thirsty"`
	Wilted      int `json:"wilted"`
	BestStreak  int `json:"bestStreak"`
}

type Garden struct {
	Date    string      `json:"date"`
	Summary Summary     `json:"summary"`
	Habits  []HabitView `json:"habits"`
}

type server struct {
	repo Repository
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/habits", s.handleHabits)
	mux.HandleFunc("/api/habits/", s.handleHabitItem)
	mux.HandleFunc("/api/garden", s.handleGarden)
	return mux
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errAlreadyChecked), errors.Is(err, errNotChecked):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "something went wrong")
	}
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func makeView(h Habit, t string) HabitView {
	return HabitView{Habit: h, Status: ComputeStatus(h.CheckIns, t)}
}

func (s *server) views() []HabitView {
	t := today()
	habits := s.repo.List()
	out := make([]HabitView, 0, len(habits))
	for _, h := range habits {
		out = append(out, makeView(h, t))
	}
	return out
}

func decodeInput(w http.ResponseWriter, r *http.Request) (habitInput, bool) {
	var in habitInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return in, false
	}
	return in, true
}

// cleanInput validates name and plant type; problem is "" when everything is fine
func cleanInput(in habitInput, requireName bool) (name, plant, problem string) {
	name = strings.TrimSpace(in.Name)
	plant = strings.ToLower(strings.TrimSpace(in.PlantType))
	if name == "" && requireName {
		return "", "", "name is required"
	}
	if len([]rune(name)) > 60 {
		return "", "", "name is too long (max 60 characters)"
	}
	if plant != "" && !plantTypes[plant] {
		return "", "", "unknown plant type (use sunflower, fern, cactus, rose or tulip)"
	}
	return name, plant, ""
}

// ---------- handlers ----------

// GET /api/habits, POST /api/habits
func (s *server) handleHabits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.views())
	case http.MethodPost:
		in, ok := decodeInput(w, r)
		if !ok {
			return
		}
		name, plant, problem := cleanInput(in, true)
		if problem != "" {
			writeErr(w, http.StatusBadRequest, problem)
			return
		}
		if plant == "" {
			plant = "sunflower"
		}
		h := Habit{ID: newID(), Name: name, PlantType: plant, CreatedAt: today(), CheckIns: []string{}}
		if err := s.repo.Add(h); err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, makeView(h, today()))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// /api/habits/{id}  and  /api/habits/{id}/checkin
func (s *server) handleHabitItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/habits/"), "/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" || len(parts) > 2 {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if len(parts) == 2 {
		if parts[1] != "checkin" {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		s.handleCheckin(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h, ok := s.repo.Get(id)
		if !ok {
			s.fail(w, ErrNotFound)
			return
		}
		writeJSON(w, http.StatusOK, makeView(h, today()))
	case http.MethodPut:
		in, ok := decodeInput(w, r)
		if !ok {
			return
		}
		name, plant, problem := cleanInput(in, false)
		if problem != "" {
			writeErr(w, http.StatusBadRequest, problem)
			return
		}
		if name == "" && plant == "" {
			writeErr(w, http.StatusBadRequest, "nothing to update")
			return
		}
		h, err := s.repo.Modify(id, func(h *Habit) error {
			if name != "" {
				h.Name = name
			}
			if plant != "" {
				h.PlantType = plant
			}
			return nil
		})
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, makeView(h, today()))
	case http.MethodDelete:
		if err := s.repo.Delete(id); err != nil {
			s.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// POST = save today's check-in, DELETE = undo it
func (s *server) handleCheckin(w http.ResponseWriter, r *http.Request, id string) {
	t := today()
	var fn func(h *Habit) error

	switch r.Method {
	case http.MethodPost:
		fn = func(h *Habit) error {
			if contains(h.CheckIns, t) {
				return errAlreadyChecked
			}
			h.CheckIns = append(h.CheckIns, t)
			return nil
		}
	case http.MethodDelete:
		fn = func(h *Habit) error {
			if !contains(h.CheckIns, t) {
				return errNotChecked
			}
			h.CheckIns = without(h.CheckIns, t)
			return nil
		}
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h, err := s.repo.Modify(id, fn)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, makeView(h, t))
}

// GET /api/garden
func (s *server) handleGarden(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	views := s.views()
	sum := Summary{TotalHabits: len(views)}
	for _, v := range views {
		if v.DoneToday {
			sum.DoneToday++
		}
		switch v.Health {
		case Healthy:
			sum.Healthy++
		case Thirsty:
			sum.Thirsty++
		case Wilted:
			sum.Wilted++
		}
		if v.LongestStreak > sum.BestStreak {
			sum.BestStreak = v.LongestStreak
		}
	}
	writeJSON(w, http.StatusOK, Garden{Date: today(), Summary: sum, Habits: views})
}

// ---------- daily refresh worker ----------

func dailyRefresh(repo Repository) {
	refresh := func() {
		t := today()
		habits := repo.List()
		wilted := 0
		for _, h := range habits {
			if ComputeStatus(h.CheckIns, t).Health == Wilted {
				wilted++
			}
		}
		log.Printf("Daily refresh (%s): %d habits, %d wilted", t, len(habits), wilted)
	}
	refresh()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	last := today()
	for range ticker.C {
		if now := today(); now != last {
			last = now
			refresh()
		}
	}
}

func main() {
	repo, err := NewJSONRepo("data/habits.json")
	if err != nil {
		log.Fatal(err)
	}
	go dailyRefresh(repo)

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", (&server{repo: repo}).routes())
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Println("Habit Garden running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
