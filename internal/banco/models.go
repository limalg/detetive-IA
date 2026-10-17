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

// TradeProposal defines property swapping between players with optional cash adjustments.
type TradeProposal struct {
	FromPlayerID   int   `json:"from_player_id"`
	ToPlayerID     int   `json:"to_player_id"`
	OfferedProps   []int `json:"offered_props"`   // tile positions offered by FromPlayer
	RequestedProps []int `json:"requested_props"` // tile positions requested from ToPlayer
	CashOffer      int   `json:"cash_offer"`      // cash given by FromPlayer to ToPlayer
	CashRequest    int   `json:"cash_request"`    // cash demanded by FromPlayer from ToPlayer
}

// TradeResult represents the outcome of a property trade negotiation.
type TradeResult struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
	Reason   string `json:"reason,omitempty"`
}

// LoanResult represents the outcome of an emergency loan from the bank.
type LoanResult struct {
	Approved bool   `json:"approved"`
	Amount   int    `json:"amount"`
	Message  string `json:"message"`
}

// RoomPlayer represents a participant waiting in a multiplayer Banco lobby.
type RoomPlayer struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Color  string `json:"color"`
	IsHost bool   `json:"is_host"`
}

// Room represents a multiplayer lobby for Banco Imobiliário.
type Room struct {
	Code        string        `json:"code"` // 4-letter uppercase code
	HostName    string        `json:"host_name"`
	City        string        `json:"city"`
	Difficulty  string        `json:"difficulty"`
	Age         int           `json:"age"`
	SchoolGrade string        `json:"school_grade"`
	Status      string        `json:"status"` // "lobby", "in_progress", "finished"
	GameID      string        `json:"game_id,omitempty"`
	Players     []*RoomPlayer `json:"players"`
	CreatedAt   string        `json:"created_at"`
}

