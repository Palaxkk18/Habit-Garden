package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var ErrNotFound = errors.New("habit not found")

// Repository separates storage from logic, so storage can be swapped or faked in tests
type Repository interface {
	List() []Habit
	Get(id string) (Habit, bool)
	Add(h Habit) error
	Modify(id string, fn func(h *Habit) error) (Habit, error)
	Delete(id string) error
}

// JSONRepo keeps habits in memory and saves them to a JSON file after every change
type JSONRepo struct {
	mu     sync.RWMutex
	path   string
	habits []Habit
}

func NewJSONRepo(path string) (*JSONRepo, error) {
	r := &JSONRepo{path: path, habits: []Habit{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &r.habits); err != nil {
			return nil, err
		}
	}
	if r.habits == nil {
		r.habits = []Habit{}
	}
	return r, nil
}

// save must be called with the write lock held
func (r *JSONRepo) save() error {
	data, err := json.MarshalIndent(r.habits, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func clone(h Habit) Habit {
	h.CheckIns = append([]string{}, h.CheckIns...)
	return h
}

func (r *JSONRepo) List() []Habit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Habit, len(r.habits))
	for i, h := range r.habits {
		out[i] = clone(h)
	}
	return out
}

func (r *JSONRepo) Get(id string) (Habit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, h := range r.habits {
		if h.ID == id {
			return clone(h), true
		}
	}
	return Habit{}, false
}

func (r *JSONRepo) Add(h Habit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.habits = append(r.habits, clone(h))
	if err := r.save(); err != nil {
		r.habits = r.habits[:len(r.habits)-1]
		return err
	}
	return nil
}

// Modify finds a habit, applies fn to a copy, and saves - all under one lock
func (r *JSONRepo) Modify(id string, fn func(h *Habit) error) (Habit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.habits {
		if r.habits[i].ID != id {
			continue
		}
		updated := clone(r.habits[i])
		if err := fn(&updated); err != nil {
			return Habit{}, err
		}
		old := r.habits[i]
		r.habits[i] = updated
		if err := r.save(); err != nil {
			r.habits[i] = old
			return Habit{}, err
		}
		return clone(updated), nil
	}
	return Habit{}, ErrNotFound
}

func (r *JSONRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.habits {
		if r.habits[i].ID == id {
			removed := r.habits[i]
			r.habits = append(r.habits[:i], r.habits[i+1:]...)
			if err := r.save(); err != nil {
				r.habits = append(r.habits[:i], append([]Habit{removed}, r.habits[i:]...)...)
				return err
			}
			return nil
		}
	}
	return ErrNotFound
}
