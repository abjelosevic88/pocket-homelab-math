package store

import (
	"encoding/json"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Notebook is a page of cells.
type Notebook struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"-"`
	Title     string   `json:"title"`
	Subject   string   `json:"subject"`
	About     string   `json:"about"`
	Color     string   `json:"color"`
	Angle     string   `json:"angle"`
	Units     bool     `json:"units"`
	Tags      []string `json:"tags"`
	Pinned    bool     `json:"pinned"`
	Archived  bool     `json:"archived"`
	Shared    bool     `json:"shared"`
	ShareKey  string   `json:"share_key"`
	Revision  int64    `json:"revision"`
	Source    string   `json:"source"`
	SourceRef string   `json:"source_ref"`
	OpenedAt  *string  `json:"opened_at"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	// Worked out when listed.
	Cells   int      `json:"cells"`
	Kinds   []string `json:"kinds"`
	Preview string   `json:"preview"`
}

// Cell is one block of a notebook.
type Cell struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"-"`
	NotebookID int64           `json:"notebook_id"`
	Position   int             `json:"position"`
	Kind       string          `json:"kind"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Data       json.RawMessage `json:"data"`
	Results    json.RawMessage `json:"results"`
	Summary    string          `json:"summary"`
	Collapsed  bool            `json:"collapsed"`
	Source     string          `json:"source"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// Formula is a saved calculation of one's own.
type Formula struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	Key       string          `json:"key,omitempty"` // built-ins only
	Name      string          `json:"name"`
	About     string          `json:"about"`
	Category  string          `json:"category"`
	Icon      string          `json:"icon"`
	Code      string          `json:"code"`
	Inputs    json.RawMessage `json:"inputs"`
	Outputs   json.RawMessage `json:"outputs"`
	FromKey   string          `json:"from_key"`
	Pinned    bool            `json:"pinned"`
	Builtin   bool            `json:"builtin"`
	Source    string          `json:"source"`
	SourceRef string          `json:"source_ref"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`

	Runs    int     `json:"runs"`
	LastRun *string `json:"last_run"`
}

// Run is a kept answer of a saved calculation.
type Run struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"-"`
	FormulaID   *int64          `json:"formula_id"`
	FormulaKey  string          `json:"formula_key"`
	FormulaName string          `json:"formula_name"`
	Title       string          `json:"title"`
	Inputs      json.RawMessage `json:"inputs"`
	Outputs     json.RawMessage `json:"outputs"`
	Main        string          `json:"main"`
	Note        string          `json:"note"`
	Source      string          `json:"source"`
	CreatedAt   string          `json:"created_at"`
}

// Practice is one round of practice.
type Practice struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	Learner   string          `json:"learner"`
	Skill     string          `json:"skill"`
	Level     int             `json:"level"`
	Questions int             `json:"questions"`
	Correct   int             `json:"correct"`
	Seconds   int             `json:"seconds"`
	Details   json.RawMessage `json:"details"`
	Day       string          `json:"day"`
	CreatedAt string          `json:"created_at"`
}

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Working out.
	Angle     string `json:"angle"`      // deg | rad: plain numbers in sin/cos/tan in a new notebook
	Units     bool   `json:"units"`      // read units and currencies in a new notebook
	Digits    int    `json:"digits"`     // significant digits shown
	NumberFmt string `json:"number_fmt"` // "1,234.5" | "1.234,5" | "1 234,5" | "1234.5"
	Currency  string `json:"currency"`   // your money, for the converter
	Rates     string `json:"rates"`      // ecb | off: exchange rates from the European Central Bank
	GraphGrid bool   `json:"graph_grid"` // grid lines in graphs
	GraphRad  bool   `json:"graph_rad"`  // graphs read x in radians
	ShowAlt   bool   `json:"show_alt"`   // fractions next to decimals

	// Practice.
	DailyGoal     int      `json:"daily_goal"`     // questions a day
	RoundSize     int      `json:"round_size"`     // questions in a round
	PracticeHabit int64    `json:"practice_habit"` // the Pocket Habits habit ticked when the goal is met
	Learners      []string `json:"learners"`       // children practising here (also from Pocket Family)

	// Other apps.
	NotesFolder string `json:"notes_folder"` // where notebooks go in Pocket Notes
	FilesFolder string `json:"files_folder"` // where pictures and tables go in Pocket Files
	ChatTo      string `json:"chat_to"`      // the Pocket Chat conversation answers are shared in
	LearnDeck   string `json:"learn_deck"`   // the Pocket Learn deck formulas go to
	AlertsGoal  bool   `json:"alerts_goal"`  // remind through Pocket Alerts when practice is not done
	AIHelp      bool   `json:"ai_help"`

	// Notifications.
	NotifyPractice bool   `json:"notify_practice"` // practice not done by the reminder hour
	NotifyHour     int    `json:"notify_hour"`     // reminders go at this hour
	NotifyURL      string `json:"notify_url"`
	NotifyFormat   string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
