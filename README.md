# Habit Garden

A Go-based habit tracker where every habit grows as a plant. Check in daily and your plant grows through 5 stages. Miss days and it gets thirsty, then wilts.

**Author:** Palak Kashyap | **Roll No:** 2401840007 | **Section:** A
**Course:** ENSP465 New Age Programming Languages Lab, Capstone Phase II
**Faculty:** Dr. Hemraj Kumawat

## Objectives
- Build a Go REST API with 7 endpoints for habits, check-ins and a garden summary
- Track current and longest streaks, with unit tests for the streak logic
- Show 5 plant growth stages plus thirsty and wilted states
- Save data in a JSON file, safe for concurrent requests
- Give a responsive browser UI for desktop and mobile

## Features
- Add, view, edit and delete habits (name and plant type: sunflower, rose, tulip, cactus, fern)
- One check-in per habit per day, with undo for today's check-in
- Current streak, longest streak and total check-ins per habit
- Plant stages by streak: seed (0), sprout (1-2), young (3-6), leafy (7-20), blooming (21+)
- Health states: healthy (checked in today), thirsty (last check-in yesterday), wilted (2+ days missed, streak resets, longest streak is kept)
- Garden summary bar (habits, done today, healthy/thirsty/wilted, best streak)
- Responsive layout (desktop and phone)
- Daily refresh worker (goroutine + time.Ticker); the browser also refreshes every minute

## Technologies Used
- **Backend:** Go standard library (net/http, encoding/json, sync, time, embed, testing, crypto/rand)
- **Frontend:** HTML, CSS, JavaScript (Fetch API)
- **Storage:** JSON file (`data/habits.json`)
- **Tools:** VS Code, Git, GitHub

## Project Structure
Habit-Garden/
├── main.go # server, routing, handlers, daily refresh worker
├── habit.go # Habit model, streak and growth engine
├── store.go # Repository interface + JSON file storage (sync.RWMutex)
├── habit_test.go # unit, API and concurrency tests
├── static/
│ └── index.html # frontend (embedded into the Go binary)
├── data/habits.json # saved habits
└── screenshots/

## REST API
| Method and path | Purpose |
|---|---|
| POST /api/habits | Add a habit |
| GET /api/habits | List all habits |
| GET /api/habits/{id} | Get one habit with streak and plant stage |
| PUT /api/habits/{id} | Edit name or plant type |
| DELETE /api/habits/{id} | Delete a habit |
| POST /api/habits/{id}/checkin | Save today's check-in |
| DELETE /api/habits/{id}/checkin | Undo today's check-in |
| GET /api/garden | All habits with computed plant state + summary |

## Implementation
- **Streak and growth engine** (`habit.go`): a pure function `ComputeStatus` takes check-in dates and today's date, and returns current streak, longest streak, stage and health. Dates are stored as YYYY-MM-DD text so streaks are counted by calendar day.
- **Storage** (`store.go`): a `Repository` interface with a JSON-file implementation. All data is protected by a `sync.RWMutex`, and the file is written safely (temp file, then rename) after every change.
- **Handlers** (`main.go`): validate input, call the repository, and return JSON with proper status codes (400, 404, 409).
- **Frontend** (`static/index.html`): draws each habit as an emoji plant that changes by stage, and updates without page reloads. It is embedded into the Go binary with `embed`.

## Testing
go test ./...
go test -v ./...
- Table-driven tests for the streak engine (13 cases: gaps, duplicates, unsorted dates, all stages, thirsty and wilted)
- Full API flow test (create, check in, undo, edit, delete, error codes)
- Concurrency test: 50 habits created at the same time, and 50 simultaneous check-ins on one habit (exactly one succeeds)

## How to Run
1. Install Go from https://go.dev/dl
2. Clone and start:
3. git clone https://github.com/Palaxkk18/Habit-Garden.git
cd Habit-Garden
go run .

3. Open http://localhost:8080

## Screenshots
![Dashboard](screenshots/dashboard.png)
![Add habit](screenshots/add-habit.png)
![Check-in](screenshots/checkin.png)
![Wilted plants](screenshots/wilted-thirsty.png)
![Tests](screenshots/tests.png)
The plant-stage in these screenshots use seeded test data (older check-in dates) because stages depend on real streaks over several days.

## Future Scope
- Plant visuals with SVG instead of emoji
- Reminders and weekly statistics

## Conclusion
Habit Garden turns habit tracking into a garden: each habit is a plant that grows with a streak and wilts when days are missed. I built it as a Go REST API with a JSON file for storage, plus a responsive browser UI, and tested the streak logic and concurrent requests. This was my first solo project in Go, and it taught me how to structure a backend, use interfaces and testcases.
