package models

import "time"

// User represents an authenticated player or detective.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an active user session token.
type Session struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Case represents a murder mystery case.
type Case struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Victim            string `json:"victim"`
	SolutionSuspectID int64  `json:"solution_suspect_id,omitempty"`
	SolutionWeapon    string `json:"solution_weapon,omitempty"`
	SolutionMotive    string `json:"solution_motive,omitempty"`
}

// Suspect represents an interrogated character in a case.
type Suspect struct {
	ID              int64  `json:"id"`
	CaseID          int64  `json:"case_id"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	Personality     string `json:"personality"`
	Alibi           string `json:"alibi"`
	Secret          string `json:"secret,omitempty"`
	IsCulprit       bool   `json:"is_culprit,omitempty"`
	Tier1Alibi      string `json:"tier1_alibi,omitempty"`
	Tier2PettyCrime string `json:"tier2_petty_crime,omitempty"`
	Tier3Truth      string `json:"tier3_truth,omitempty"`
	CrossSuspicion  string `json:"cross_suspicion,omitempty"`
}

// Clue represents physical evidence found at a crime scene.
type Clue struct {
	ID          int64  `json:"id"`
	CaseID      int64  `json:"case_id"`
	Location    string `json:"location"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// InterrogationMessage represents a chat exchange between detective and suspect.
type InterrogationMessage struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	SuspectID int64     `json:"suspect_id"`
	Sender    string    `json:"sender"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Investigation tracks the state and resolution of a case by a user.
type Investigation struct {
	ID                   int64      `json:"id"`
	UserID               int64      `json:"user_id"`
	CaseID               int64      `json:"case_id"`
	Status               string     `json:"status"` // in_progress, solved, failed
	TimeRemainingMinutes int        `json:"time_remaining_minutes"` // default 420 (21:00 to 04:00)
	WarrantsRemaining    int        `json:"warrants_remaining"`     // default 2
	AccusedSuspectID     *int64     `json:"accused_suspect_id,omitempty"`
	AccusedWeapon        string     `json:"accused_weapon,omitempty"`
	AccusedMotive        string     `json:"accused_motive,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
}

// Verdict represents the outcome of an accusation.
type Verdict struct {
	Correct       bool   `json:"correct"`
	Score         int    `json:"score"`
	Feedback      string `json:"feedback"`
	ActualCulprit string `json:"actual_culprit,omitempty"`
	ActualWeapon  string `json:"actual_weapon,omitempty"`
	ActualMotive  string `json:"actual_motive,omitempty"`
}
