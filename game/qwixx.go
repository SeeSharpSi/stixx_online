package game

import (
	"fmt"
	"seesharpsi/stixx_online/db"
)

// Row represents the numbers available in each color
type Row struct {
	Color   string
	Numbers []int
	Locked  bool
}

// GameState represents the current state of a Qwixx game
type GameState struct {
	Game    *db.Game
	Players []db.Player
	Rows    map[string]Row
}

// Initialize the rows for Qwixx
func InitializeRows() map[string]Row {
	rows := make(map[string]Row)

	// Red and Yellow go from 2 to 12
	redNumbers := make([]int, 11)
	yellowNumbers := make([]int, 11)
	for i := 0; i < 11; i++ {
		redNumbers[i] = i + 2
		yellowNumbers[i] = i + 2
	}

	// Green and Blue go from 12 to 2
	greenNumbers := make([]int, 11)
	blueNumbers := make([]int, 11)
	for i := 0; i < 11; i++ {
		greenNumbers[i] = 12 - i
		blueNumbers[i] = 12 - i
	}

	rows["red"] = Row{Color: "red", Numbers: redNumbers, Locked: false}
	rows["yellow"] = Row{Color: "yellow", Numbers: yellowNumbers, Locked: false}
	rows["green"] = Row{Color: "green", Numbers: greenNumbers, Locked: false}
	rows["blue"] = Row{Color: "blue", Numbers: blueNumbers, Locked: false}

	return rows
}

// LoadGameState loads the current game state from the database
func LoadGameState(gameCode string) (*GameState, error) {
	game, err := db.GetGame(gameCode)
	if err != nil {
		return nil, err
	}

	players, err := db.GetPlayers(game.ID)
	if err != nil {
		return nil, err
	}

	rows := InitializeRows()
	redRow := rows["red"]
	redRow.Locked = game.RedLocked
	rows["red"] = redRow

	yellowRow := rows["yellow"]
	yellowRow.Locked = game.YellowLocked
	rows["yellow"] = yellowRow

	greenRow := rows["green"]
	greenRow.Locked = game.GreenLocked
	rows["green"] = greenRow

	blueRow := rows["blue"]
	blueRow.Locked = game.BlueLocked
	rows["blue"] = blueRow

	return &GameState{
		Game:    game,
		Players: players,
		Rows:    rows,
	}, nil
}

// GetPlayerMarkedNumbers returns all numbers marked by a player organized by color
func GetPlayerMarkedNumbers(playerID int) (map[string][]int, error) {
	marks, err := db.GetPlayerMarks(playerID)
	if err != nil {
		return nil, err
	}

	markedNumbers := make(map[string][]int)
	for _, mark := range marks {
		markedNumbers[mark.Color] = append(markedNumbers[mark.Color], mark.Number)
	}

	return markedNumbers, nil
}

// IsValidMark checks if a player can mark a specific number
func IsValidMark(playerID int, color string, number int, rows map[string]Row) (bool, error) {
	row := rows[color]

	// Can't mark in a locked row
	if row.Locked {
		return false, nil
	}

	// Check if number exists in this row
	validNumber := false
	numberIndex := -1
	for i, n := range row.Numbers {
		if n == number {
			validNumber = true
			numberIndex = i
			break
		}
	}

	if !validNumber {
		return false, nil
	}

	// Get player's existing marks
	markedNumbers, err := GetPlayerMarkedNumbers(playerID)
	if err != nil {
		return false, err
	}

	colorMarks := markedNumbers[color]

	// If no marks in this color yet, any number is valid
	if len(colorMarks) == 0 {
		// Special rule: can only mark rightmost number (lock) if you have at least 5 marks
		if numberIndex == len(row.Numbers)-1 {
			return false, nil
		}
		return true, nil
	}

	// Find the rightmost marked number
	rightmostIndex := -1
	for _, markedNum := range colorMarks {
		for i, n := range row.Numbers {
			if n == markedNum && i > rightmostIndex {
				rightmostIndex = i
			}
		}
	}

	// Can only mark numbers to the right of the rightmost mark
	if numberIndex <= rightmostIndex {
		return false, nil
	}

	// Special rule for locking: need at least 5 marks to mark the rightmost number
	if numberIndex == len(row.Numbers)-1 && len(colorMarks) < 5 {
		return false, nil
	}

	return true, nil
}

// GetPossibleMoves returns all valid moves for a player given the current dice
func GetPossibleMoves(playerID int, game *db.Game, isActivePlayer bool) ([]Move, error) {
	// No moves if dice haven't been rolled
	if !game.DiceRolled {
		return []Move{}, nil
	}

	rows := InitializeRows()
	redRow := rows["red"]
	redRow.Locked = game.RedLocked
	rows["red"] = redRow

	yellowRow := rows["yellow"]
	yellowRow.Locked = game.YellowLocked
	rows["yellow"] = yellowRow

	greenRow := rows["green"]
	greenRow.Locked = game.GreenLocked
	rows["green"] = greenRow

	blueRow := rows["blue"]
	blueRow.Locked = game.BlueLocked
	rows["blue"] = blueRow

	var moves []Move

	// All players can use the sum of white dice (if not already used)
	if !game.WhiteMarkUsed {
		whiteSum := game.WhiteDice1 + game.WhiteDice2

		for color := range rows {
			if valid, _ := IsValidMark(playerID, color, whiteSum, rows); valid {
				moves = append(moves, Move{
					PlayerID: playerID,
					Color:    color,
					Number:   whiteSum,
					Type:     "white",
				})
			}
		}
	}

	// Active player can also use white + colored dice (if not already used)
	if isActivePlayer && !game.ColoredMarkUsed {
		colorDice := map[string]int{
			"red":    game.RedDice,
			"yellow": game.YellowDice,
			"green":  game.GreenDice,
			"blue":   game.BlueDice,
		}

		for color, colorValue := range colorDice {
			// White1 + Color
			sum1 := game.WhiteDice1 + colorValue
			if valid, _ := IsValidMark(playerID, color, sum1, rows); valid {
				moves = append(moves, Move{
					PlayerID: playerID,
					Color:    color,
					Number:   sum1,
					Type:     "colored",
				})
			}

			// White2 + Color
			sum2 := game.WhiteDice2 + colorValue
			if sum2 != sum1 { // Avoid duplicates
				if valid, _ := IsValidMark(playerID, color, sum2, rows); valid {
					moves = append(moves, Move{
						PlayerID: playerID,
						Color:    color,
						Number:   sum2,
						Type:     "colored",
					})
				}
			}
		}
	}

	return moves, nil
}

// Move represents a possible move in the game
type Move struct {
	PlayerID int
	Color    string
	Number   int
	Type     string // "white" or "colored"
}

// MakeMark processes a player marking a number
func MakeMark(playerID int, color string, number int, gameID int, moveType string) error {
	// First get the full game state
	var game db.Game
	err := db.DB.QueryRow(`SELECT id, game_code, status, red_locked, yellow_locked, green_locked, blue_locked,
		white_mark_used, colored_mark_used, current_player_index, dice_rolled
		FROM games WHERE id = ?`, gameID).Scan(&game.ID, &game.GameCode, &game.Status, &game.RedLocked,
		&game.YellowLocked, &game.GreenLocked, &game.BlueLocked, &game.WhiteMarkUsed, &game.ColoredMarkUsed,
		&game.CurrentPlayerIndex, &game.DiceRolled)
	if err != nil {
		return err
	}

	// Check if dice have been rolled
	if !game.DiceRolled {
		return fmt.Errorf("dice have not been rolled")
	}

	// Check if this type of move has already been used
	if moveType == "white" && game.WhiteMarkUsed {
		return fmt.Errorf("white dice move already used this turn")
	}
	if moveType == "colored" && game.ColoredMarkUsed {
		return fmt.Errorf("colored dice move already used this turn")
	}

	// Get current player
	currentPlayer, err := GetCurrentPlayer(gameID)
	if err != nil {
		return err
	}

	// Check if it's a colored move and player is not active
	if moveType == "colored" && currentPlayer.ID != playerID {
		return fmt.Errorf("only active player can use colored dice")
	}

	rows := InitializeRows()
	redRow := rows["red"]
	redRow.Locked = game.RedLocked
	rows["red"] = redRow

	yellowRow := rows["yellow"]
	yellowRow.Locked = game.YellowLocked
	rows["yellow"] = yellowRow

	greenRow := rows["green"]
	greenRow.Locked = game.GreenLocked
	rows["green"] = greenRow

	blueRow := rows["blue"]
	blueRow.Locked = game.BlueLocked
	rows["blue"] = blueRow

	valid, err := IsValidMark(playerID, color, number, rows)
	if err != nil {
		return err
	}

	if !valid {
		return fmt.Errorf("invalid move")
	}

	// Mark the number
	err = db.MarkNumber(playerID, color, number)
	if err != nil {
		return err
	}

	// Update which move type was used
	if moveType == "white" {
		_, err = db.DB.Exec("UPDATE games SET white_mark_used = TRUE WHERE id = ?", gameID)
	} else if moveType == "colored" {
		_, err = db.DB.Exec("UPDATE games SET colored_mark_used = TRUE WHERE id = ?", gameID)
	}
	if err != nil {
		return err
	}

	// Check if this locks the row (marking rightmost number)
	row := rows[color]
	if number == row.Numbers[len(row.Numbers)-1] {
		err = db.LockColor(gameID, color)
		if err != nil {
			return err
		}
	}

	// Check if turn should automatically end for active player
	if currentPlayer.ID == playerID {
		// Active player - check if both moves used
		var whiteUsed, coloredUsed bool
		err = db.DB.QueryRow("SELECT white_mark_used, colored_mark_used FROM games WHERE id = ?", gameID).Scan(&whiteUsed, &coloredUsed)
		if err == nil && whiteUsed && coloredUsed {
			// Both moves used - automatically end turn
			// Move to next player and reset turn state
			err = db.NextTurn(gameID)
			if err != nil {
				return fmt.Errorf("failed to end turn: %w", err)
			}

			// Check if game is finished
			finished, err := db.IsGameFinished(gameID)
			if err == nil && finished {
				db.DB.Exec("UPDATE games SET status = 'finished' WHERE id = ?", gameID)
			}
		}
	}

	return nil
}

// CalculateScore calculates a player's score
func CalculateScore(playerID int) (int, error) {
	marks, err := db.GetPlayerMarks(playerID)
	if err != nil {
		return 0, err
	}

	// Count marks per color
	colorCounts := make(map[string]int)
	for _, mark := range marks {
		colorCounts[mark.Color]++
	}

	// Score table for Qwixx
	scoreTable := []int{0, 1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 66, 78}

	totalScore := 0
	for _, count := range colorCounts {
		if count < len(scoreTable) {
			totalScore += scoreTable[count]
		} else {
			totalScore += scoreTable[len(scoreTable)-1]
		}
	}

	// Get penalties
	var player db.Player
	err = db.DB.QueryRow("SELECT penalties FROM players WHERE id = ?", playerID).Scan(&player.Penalties)
	if err != nil {
		return 0, err
	}

	// Subtract 5 points per penalty
	totalScore -= player.Penalties * 5

	return totalScore, nil
}

// GetCurrentPlayer returns the current player in a game
func GetCurrentPlayer(gameID int) (*db.Player, error) {
	var player db.Player
	err := db.DB.QueryRow(`
		SELECT p.id, p.game_id, p.name, p.turn_order, p.joined_at, p.penalties, p.is_active
		FROM players p
		JOIN games g ON g.id = p.game_id
		WHERE g.id = ? AND p.turn_order = g.current_player_index
	`, gameID).Scan(&player.ID, &player.GameID, &player.Name, &player.TurnOrder,
		&player.JoinedAt, &player.Penalties, &player.IsActive)

	if err != nil {
		return nil, err
	}

	return &player, nil
}

// ProcessTurn handles the logic for processing a turn
func ProcessTurn(gameID int, playerMoves map[int][]Move, skipPenalty bool) error {
	// Get current player
	currentPlayer, err := GetCurrentPlayer(gameID)
	if err != nil {
		return err
	}

	// Process moves for all players
	for playerID, moves := range playerMoves {
		for _, move := range moves {
			err := MakeMark(playerID, move.Color, move.Number, gameID, move.Type)
			if err != nil {
				return err
			}
		}
	}

	// If active player didn't make any colored move, they get a penalty
	if !skipPenalty {
		activeMoves := playerMoves[currentPlayer.ID]
		hasColoredMove := false
		for _, move := range activeMoves {
			if move.Type == "colored" {
				hasColoredMove = true
				break
			}
		}

		if !hasColoredMove {
			err = db.AddPenalty(currentPlayer.ID)
			if err != nil {
				return err
			}
		}
	}

	// Check if game is finished
	finished, err := db.IsGameFinished(gameID)
	if err != nil {
		return err
	}

	if finished {
		_, err = db.DB.Exec("UPDATE games SET status = 'finished' WHERE id = ?", gameID)
		return err
	}

	// Move to next player
	return db.NextTurn(gameID)
}
