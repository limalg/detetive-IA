package banco3d

import (
	"strings"
)

// BotDifficulty represents the intelligence level of an automated player.
type BotDifficulty string

const (
	BotDifficultyEasy   BotDifficulty = "easy"
	BotDifficultyNormal BotDifficulty = "normal"
	BotDifficultyHard   BotDifficulty = "hard"
)

// BotParams holds tuning parameters per difficulty.
type BotParams struct {
	MinReserve      int
	NoiseRate       float64
	TradeThreshold  float64
	CanProposeTrade bool
	SqueezeMargin   bool
}

// GetBotParams returns the canonical decision parameters for a difficulty.
func GetBotParams(diff string) BotParams {
	switch strings.ToLower(diff) {
	case "easy":
		return BotParams{
			MinReserve:      100,
			NoiseRate:       0.25,
			TradeThreshold:  0.70,
			CanProposeTrade: false,
			SqueezeMargin:   false,
		}
	case "hard":
		return BotParams{
			MinReserve:      400,
			NoiseRate:       0.0,
			TradeThreshold:  1.15,
			CanProposeTrade: true,
			SqueezeMargin:   true,
		}
	default: // normal / medium
		return BotParams{
			MinReserve:      250,
			NoiseRate:       0.10,
			TradeThreshold:  1.00,
			CanProposeTrade: true,
			SqueezeMargin:   false,
		}
	}
}

// BotDecisionEngine provides AI evaluation for automated players.
type BotDecisionEngine struct{}

var BotAI = &BotDecisionEngine{}

// ShouldBuy decides if a bot should purchase the unowned tile it landed on.
func (b *BotDecisionEngine) ShouldBuy(g *Game, p *Player, tile *Tile, diff string) bool {
	if tile == nil || tile.Price <= 0 || tile.OwnerID != "" {
		return false
	}
	if p.Money < tile.Price {
		return false
	}

	params := GetBotParams(diff)

	// Check if this property completes a color group for player p
	completesGroup := false
	if tile.Group != ColorNone {
		completesGroup = true
		for _, idx := range g.Board.GetGroupIndices(tile.Group) {
			if idx == tile.Index {
				continue
			}
			other := &g.Board.Tiles[idx]
			if other.OwnerID != p.ID {
				completesGroup = false
				break
			}
		}
	}

	remaining := p.Money - tile.Price
	effectiveReserve := params.MinReserve

	// Squeeze rule for Hard difficulty: ignore 50% of the reserve if completing a group
	if params.SqueezeMargin && completesGroup {
		effectiveReserve = params.MinReserve / 2
	}

	if remaining < effectiveReserve {
		return false
	}

	// Never buy below $60 if remaining cash < 2 * reserve
	if tile.Price < 60 && remaining < params.MinReserve*2 {
		return false
	}

	return true
}

// DecideBuild determines which tile to construct a house/hotel on.
func (b *BotDecisionEngine) DecideBuild(g *Game, p *Player, diff string) (int, bool) {
	params := GetBotParams(diff)

	multiplier := 1.5
	if diff == "easy" {
		multiplier = 2.0
	} else if diff == "hard" {
		multiplier = 1.0
	}

	requiredCash := int(float64(params.MinReserve) * multiplier)

	groups := []ColorGroup{
		ColorDarkBlue, ColorGreen, ColorYellow, ColorRed,
		ColorOrange, ColorPink, ColorLightBlue, ColorBrown,
	}

	for _, grp := range groups {
		indices := g.Board.GetGroupIndices(grp)
		if len(indices) == 0 {
			continue
		}

		// Verify complete group ownership and absence of mortgage
		allOwned := true
		minHouses := 5
		for _, idx := range indices {
			t := &g.Board.Tiles[idx]
			if t.OwnerID != p.ID || t.IsMortgaged {
				allOwned = false
				break
			}
			if t.Houses < minHouses {
				minHouses = t.Houses
			}
		}
		if !allOwned || minHouses >= 5 {
			continue
		}

		houseCost := g.Board.Tiles[indices[0]].HouseCost
		if p.Money < houseCost+requiredCash {
			continue
		}

		// Find the tile with minHouses to build evenly
		for _, idx := range indices {
			t := &g.Board.Tiles[idx]
			if t.Houses == minHouses && t.Houses < 5 {
				return idx, true
			}
		}
	}

	return -1, false
}

// DecideMortgage picks an unbuilt property to mortgage when low on funds.
func (b *BotDecisionEngine) DecideMortgage(g *Game, p *Player) (int, bool) {
	// First: sell houses if any
	for _, idx := range p.Properties {
		t := &g.Board.Tiles[idx]
		if t.Houses > 0 {
			return idx, true // sell house
		}
	}

	// Second: mortgage properties that do not belong to a complete group
	for _, idx := range p.Properties {
		t := &g.Board.Tiles[idx]
		if !t.IsMortgaged && t.Houses == 0 {
			return idx, true
		}
	}

	return -1, false
}

// EvaluateTrade decides whether a bot accepts an incoming trade proposal.
func (b *BotDecisionEngine) EvaluateTrade(g *Game, p *Player, proposal TradeProposal, diff string) bool {
	params := GetBotParams(diff)

	// Calculate perceived value offered vs requested
	offeredValue := proposal.OfferedCash
	for _, idx := range proposal.OfferedTiles {
		if idx >= 0 && idx < len(g.Board.Tiles) {
			t := &g.Board.Tiles[idx]
			offeredValue += t.Price + t.Rent[0]*2
		}
	}

	requestedValue := proposal.RequestedCash
	for _, idx := range proposal.RequestedTiles {
		if idx >= 0 && idx < len(g.Board.Tiles) {
			t := &g.Board.Tiles[idx]
			requestedValue += t.Price + t.Rent[0]*2
		}
	}

	// Check if this proposal takes money leaving bot in debt
	if p.Money+(proposal.OfferedCash-proposal.RequestedCash) < 0 {
		return false
	}

	ratio := float64(offeredValue) / float64(max(1, requestedValue))
	return ratio >= params.TradeThreshold
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
