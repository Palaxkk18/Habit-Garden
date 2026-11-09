package main

import (
	"sort"
	"time"
)

const dateLayout = "2006-01-02"

type PlantStage string

const (
	StageSeed     PlantStage = "seed"
	StageSprout   PlantStage = "sprout"
	StageYoung    PlantStage = "young"
	StageLeafy    PlantStage = "leafy"
	StageBlooming PlantStage = "blooming"
)

type Health string

const (
	Healthy Health = "healthy"
	Thirsty Health = "thirsty"
	Wilted  Health = "wilted"
)

// Habit is what gets saved in habits.json
type Habit struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	PlantType string   `json:"plantType"`
	CreatedAt string   `json:"createdAt"`
	CheckIns  []string `json:"checkIns"`
}

// Status is computed from check-in dates (never stored)
type Status struct {
	CurrentStreak int        `json:"currentStreak"`
	LongestStreak int        `json:"longestStreak"`
	TotalCheckIns int        `json:"totalCheckIns"`
	Stage         PlantStage `json:"stage"`
	Health        Health     `json:"health"`
	DoneToday     bool       `json:"doneToday"`
}

func today() string { return time.Now().Format(dateLayout) }

// dayDiff returns how many days `to` is after `from`
func dayDiff(from, to string) int {
	a, _ := time.Parse(dateLayout, from)
	b, _ := time.Parse(dateLayout, to)
	return int(b.Sub(a).Hours() / 24)
}

func stageFor(streak int) PlantStage {
	switch {
	case streak >= 21:
		return StageBlooming
	case streak >= 7:
		return StageLeafy
	case streak >= 3:
		return StageYoung
	case streak >= 1:
		return StageSprout
	default:
		return StageSeed
	}
}

func uniqueSorted(dates []string) []string {
	cp := append([]string{}, dates...)
	sort.Strings(cp) // YYYY-MM-DD sorts correctly as text
	out := make([]string, 0, len(cp))
	for i, d := range cp {
		if i == 0 || d != cp[i-1] {
			out = append(out, d)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func without(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// ComputeStatus is the streak and growth engine (pure function, easy to test).
// gap = days between today and the last check-in:
//
//	0 -> healthy, 1 -> thirsty (streak still alive), 2+ -> wilted (streak resets to 0)
func ComputeStatus(checkIns []string, todayStr string) Status {
	dates := uniqueSorted(checkIns)
	st := Status{TotalCheckIns: len(dates)}

	if len(dates) == 0 {
		st.Stage = StageSeed
		st.Health = Healthy
		return st
	}

	// longest streak = longest run of consecutive days
	run, longest := 1, 1
	for i := 1; i < len(dates); i++ {
		if dayDiff(dates[i-1], dates[i]) == 1 {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	st.LongestStreak = longest

	// after the loop, `run` is the length of the run that ends at the last check-in
	gap := dayDiff(dates[len(dates)-1], todayStr)
	if gap < 0 {
		gap = 0
	}
	st.DoneToday = gap == 0

	switch {
	case gap == 0:
		st.CurrentStreak = run
		st.Health = Healthy
	case gap == 1:
		st.CurrentStreak = run
		st.Health = Thirsty
	default:
		st.CurrentStreak = 0
		st.Health = Wilted
	}
	st.Stage = stageFor(st.CurrentStreak)
	return st
}
