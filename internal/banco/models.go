package banco

// Tile types supported by Banco Imobiliário IA.
const (
	TileStart    = "START"
	TileProperty = "PROPERTY"
	TileStation  = "STATION"
	TileUtility  = "UTILITY"
	TileLuck     = "LUCK"
	TileTax      = "TAX"
	TileJail     = "JAIL"
	TileGoToJail = "GO_TO_JAIL"
	TileParking  = "PARKING"
)

// Tile represents a square on the 28-tile board.
type Tile struct {
	Position  int    `json:"position"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Group     string `json:"group"` // "brown", "light_blue", "pink", "orange", "red", "yellow", "green", "dark_blue", "station", "utility", "special"
	Price     int    `json:"price"`
	RentBase  int    `json:"rent_base"`
	RentHouse [5]int `json:"rent_house"` // [0..3]=1..4 houses, [4]=hotel
	BuildCost int    `json:"build_cost"`
	OwnerID   int    `json:"owner_id"` // 0 = unowned/bank
	Houses    int    `json:"houses"`   // 0..4
	Hotel     bool   `json:"hotel"`
}

// Player represents a human or autonomous AI participant.
type Player struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	IsBot      bool   `json:"is_bot"`
	Color      string `json:"color"`
	Avatar     string `json:"avatar"`
	Balance    int    `json:"balance"`
	Position   int    `json:"position"`
	InJail     bool   `json:"in_jail"`
	JailTurns  int    `json:"jail_turns"`
	Bankrupt   bool   `json:"bankrupt"`
	Properties []int  `json:"properties"` // list of tile positions owned
}

// Card represents a Luck (Sorte) or Setback (Revés) event.
type Card struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "luck", "setback"
	Title       string `json:"title"`
	Description string `json:"description"`
	Amount      int    `json:"amount"` // + or - currency
	MoveTo      *int   `json:"move_to,omitempty"`
}

// GameState holds the synchronized board game state.
type GameState struct {
	ID           string    `json:"id"`
	City         string    `json:"city"`
	Age          int       `json:"age"`
	SchoolGrade  string    `json:"school_grade"`
	Difficulty   string    `json:"difficulty"`
	CurrentTurn  int       `json:"current_turn"` // ID of the player whose turn it is
	Round        int       `json:"round"`
	Players      []*Player `json:"players"`
	Board        []Tile    `json:"board"`
	History      []string  `json:"history"`
	Status       string    `json:"status"` // "in_progress", "finished"
	Winner       string    `json:"winner,omitempty"`
	LastDice     [2]int    `json:"last_dice"`
	DoublesCount int       `json:"doubles_count"`
	CanRoll      bool      `json:"can_roll"`
	CanBuy       bool      `json:"can_buy"`
	CanEndTurn   bool      `json:"can_end_turn"`
}

// GameOptions defines startup configuration.
type GameOptions struct {
	City            string   `json:"city"`
	Age             int      `json:"age"`
	SchoolGrade     string   `json:"school_grade"`
	Difficulty      string   `json:"difficulty"`
	BotsCount       int      `json:"bots_count"`
	StartingBalance int      `json:"starting_balance"`
	HumanName       string   `json:"human_name"`
	Friends         []string `json:"friends"`
}

// DiceRollResult conveys dice and movement event metrics.
type DiceRollResult struct {
	Dice        [2]int `json:"dice"`
	Total       int    `json:"total"`
	IsDouble    bool   `json:"is_double"`
	NewPosition int    `json:"new_position"`
	PassedStart bool   `json:"passed_start"`
	EventLog    string `json:"event_log"`
	CanBuy      bool   `json:"can_buy"`
	RentPaid    int    `json:"rent_paid,omitempty"`
	RentTo      int    `json:"rent_to,omitempty"`
}
