package db

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func init() {
	rand.Seed(time.Now().UnixNano())
}

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func InitDB() error {
	var err error
	DB, err = sql.Open("sqlite3", "./qwixx.db")
	if err != nil {
		return err
	}

	// Create tables
	createTablesSQL := `
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_code TEXT UNIQUE NOT NULL,
		status TEXT DEFAULT 'waiting', -- waiting, active, finished
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		current_player_index INTEGER DEFAULT 0,
		white_dice_1 INTEGER DEFAULT 0,
		white_dice_2 INTEGER DEFAULT 0,
		red_dice INTEGER DEFAULT 0,
		yellow_dice INTEGER DEFAULT 0,
		green_dice INTEGER DEFAULT 0,
		blue_dice INTEGER DEFAULT 0,
		red_locked BOOLEAN DEFAULT FALSE,
		yellow_locked BOOLEAN DEFAULT FALSE,
		green_locked BOOLEAN DEFAULT FALSE,
		blue_locked BOOLEAN DEFAULT FALSE,
		penalties_triggered INTEGER DEFAULT 0,
		dice_rolled BOOLEAN DEFAULT FALSE,
		white_mark_used BOOLEAN DEFAULT FALSE,
		colored_mark_used BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		turn_order INTEGER DEFAULT 0,
		joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		penalties INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT TRUE,
		FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
		UNIQUE(game_id, name)
	);

	CREATE TABLE IF NOT EXISTS player_marks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id INTEGER NOT NULL,
		color TEXT NOT NULL, -- red, yellow, green, blue
		number INTEGER NOT NULL,
		marked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
		UNIQUE(player_id, color, number)
	);

	CREATE INDEX IF NOT EXISTS idx_games_code ON games(game_code);
	CREATE INDEX IF NOT EXISTS idx_players_game ON players(game_id);
	CREATE INDEX IF NOT EXISTS idx_marks_player ON player_marks(player_id);
	`

	_, err = DB.Exec(createTablesSQL)
	return err
}

func GenerateGameCode() (string, error) {
	for attempts := 0; attempts < 100; attempts++ {
		code := make([]byte, 5)
		for i := range code {
			code[i] = charset[rand.Intn(len(charset))]
		}

		gameCode := string(code)

		// Check if code already exists
		var exists bool
		err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM games WHERE game_code = ?)", gameCode).Scan(&exists)
		if err != nil {
			return "", err
		}

		if !exists {
			return gameCode, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique game code")
}

type Game struct {
	ID                 int
	GameCode           string
	Status             string
	CreatedAt          time.Time
	CurrentPlayerIndex int
	WhiteDice1         int
	WhiteDice2         int
	RedDice            int
	YellowDice         int
	GreenDice          int
	BlueDice           int
	RedLocked          bool
	YellowLocked       bool
	GreenLocked        bool
	BlueLocked         bool
	PenaltiesTriggered int
	DiceRolled         bool
	WhiteMarkUsed      bool
	ColoredMarkUsed    bool
}

type Player struct {
	ID        int
	GameID    int
	Name      string
	TurnOrder int
	JoinedAt  time.Time
	Penalties int
	IsActive  bool
}

type PlayerMark struct {
	ID       int
	PlayerID int
	Color    string
	Number   int
	MarkedAt time.Time
}

func CreateGame() (*Game, error) {
	gameCode, err := GenerateGameCode()
	if err != nil {
		return nil, err
	}

	result, err := DB.Exec("INSERT INTO games (game_code) VALUES (?)", gameCode)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	game := &Game{
		ID:       int(id),
		GameCode: gameCode,
		Status:   "waiting",
	}

	return game, nil
}

func GetGame(gameCode string) (*Game, error) {
	game := &Game{}
	err := DB.QueryRow(`
		SELECT id, game_code, status, created_at, current_player_index,
		       white_dice_1, white_dice_2, red_dice, yellow_dice, green_dice, blue_dice,
		       red_locked, yellow_locked, green_locked, blue_locked, penalties_triggered,
		       dice_rolled, white_mark_used, colored_mark_used
		FROM games WHERE game_code = ?
	`, gameCode).Scan(
		&game.ID, &game.GameCode, &game.Status, &game.CreatedAt, &game.CurrentPlayerIndex,
		&game.WhiteDice1, &game.WhiteDice2, &game.RedDice, &game.YellowDice, &game.GreenDice, &game.BlueDice,
		&game.RedLocked, &game.YellowLocked, &game.GreenLocked, &game.BlueLocked, &game.PenaltiesTriggered,
		&game.DiceRolled, &game.WhiteMarkUsed, &game.ColoredMarkUsed,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}

	return game, err
}

func JoinGame(gameCode, playerName string) (*Player, error) {
	// Get game
	game, err := GetGame(gameCode)
	if err != nil {
		return nil, err
	}

	// Check for existing player with that name
	var existingPlayer Player
	err = DB.QueryRow(`
		SELECT id, game_id, name, turn_order, joined_at, penalties, is_active
		FROM players WHERE game_id = ? AND name = ?
	`, game.ID, playerName).Scan(
		&existingPlayer.ID, &existingPlayer.GameID, &existingPlayer.Name, &existingPlayer.TurnOrder,
		&existingPlayer.JoinedAt, &existingPlayer.Penalties, &existingPlayer.IsActive,
	)

	if err == nil {
		// Player found, return them (rejoin)
		return &existingPlayer, nil
	}

	if err != sql.ErrNoRows {
		// A different database error occurred
		return nil, err
	}

	// Player does not exist, check if game is open for new players
	if game.Status != "waiting" {
		return nil, fmt.Errorf("game has already started, so you can't join with a new name")
	}

	// Count existing players to determine turn order for new player
	var playerCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM players WHERE game_id = ?", game.ID).Scan(&playerCount)
	if err != nil {
		return nil, err
	}

	// Create new player
	result, err := DB.Exec(
		"INSERT INTO players (game_id, name, turn_order) VALUES (?, ?, ?)",
		game.ID, playerName, playerCount,
	)
	if err != nil {
		return nil, err
	}

	playerID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	player := &Player{
		ID:        int(playerID),
		GameID:    game.ID,
		Name:      playerName,
		TurnOrder: playerCount,
	}

	return player, nil
}

func GetPlayers(gameID int) ([]Player, error) {
	rows, err := DB.Query(`
		SELECT id, game_id, name, turn_order, joined_at, penalties, is_active
		FROM players WHERE game_id = ? ORDER BY turn_order
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		err := rows.Scan(&p.ID, &p.GameID, &p.Name, &p.TurnOrder, &p.JoinedAt, &p.Penalties, &p.IsActive)
		if err != nil {
			return nil, err
		}
		players = append(players, p)
	}

	return players, nil
}

func GetPlayerMarks(playerID int) ([]PlayerMark, error) {
	rows, err := DB.Query(`
		SELECT id, player_id, color, number, marked_at
		FROM player_marks WHERE player_id = ? ORDER BY color, number
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var marks []PlayerMark
	for rows.Next() {
		var m PlayerMark
		err := rows.Scan(&m.ID, &m.PlayerID, &m.Color, &m.Number, &m.MarkedAt)
		if err != nil {
			return nil, err
		}
		marks = append(marks, m)
	}

	return marks, nil
}

func RollDice(gameID int) error {
	white1 := rand.Intn(6) + 1
	white2 := rand.Intn(6) + 1
	red := rand.Intn(6) + 1
	yellow := rand.Intn(6) + 1
	green := rand.Intn(6) + 1
	blue := rand.Intn(6) + 1

	_, err := DB.Exec(`
		UPDATE games
		SET white_dice_1 = ?, white_dice_2 = ?, red_dice = ?,
		    yellow_dice = ?, green_dice = ?, blue_dice = ?,
		    dice_rolled = TRUE
		WHERE id = ?
	`, white1, white2, red, yellow, green, blue, gameID)

	return err
}

func StartGame(gameID int) error {
	_, err := DB.Exec("UPDATE games SET status = 'active' WHERE id = ?", gameID)
	return err
}

func MarkNumber(playerID int, color string, number int) error {
	// Validate that this is a legal move
	// TODO: Add validation logic based on Qwixx rules

	_, err := DB.Exec(
		"INSERT INTO player_marks (player_id, color, number) VALUES (?, ?, ?)",
		playerID, color, number,
	)

	return err
}

func NextTurn(gameID int) error {
	// Get current player index and total players
	var currentIndex, playerCount int
	err := DB.QueryRow(`
		SELECT g.current_player_index, COUNT(p.id)
		FROM games g
		JOIN players p ON p.game_id = g.id
		WHERE g.id = ?
		GROUP BY g.current_player_index
	`, gameID).Scan(&currentIndex, &playerCount)

	if err != nil {
		return err
	}

	// Move to next player and reset turn state
	nextIndex := (currentIndex + 1) % playerCount

	_, err = DB.Exec(`UPDATE games
		SET current_player_index = ?,
		    dice_rolled = FALSE,
		    white_mark_used = FALSE,
		    colored_mark_used = FALSE
		WHERE id = ?`, nextIndex, gameID)
	return err
}

func AddPenalty(playerID int) error {
	_, err := DB.Exec("UPDATE players SET penalties = penalties + 1 WHERE id = ?", playerID)
	return err
}

func LockColor(gameID int, color string) error {
	query := fmt.Sprintf("UPDATE games SET %s_locked = TRUE WHERE id = ?", color)
	_, err := DB.Exec(query, gameID)
	return err
}

func IsGameFinished(gameID int) (bool, error) {
	// Game ends when 2 colors are locked or a player has 4 penalties
	var lockedCount int
	err := DB.QueryRow(`
		SELECT (CASE WHEN red_locked THEN 1 ELSE 0 END) +
		       (CASE WHEN yellow_locked THEN 1 ELSE 0 END) +
		       (CASE WHEN green_locked THEN 1 ELSE 0 END) +
		       (CASE WHEN blue_locked THEN 1 ELSE 0 END)
		FROM games WHERE id = ?
	`, gameID).Scan(&lockedCount)

	if err != nil {
		return false, err
	}

	if lockedCount >= 2 {
		return true, nil
	}

	// Check if any player has 4 penalties
	var maxPenalties int
	err = DB.QueryRow("SELECT MAX(penalties) FROM players WHERE game_id = ?", gameID).Scan(&maxPenalties)
	if err != nil {
		return false, err
	}

	return maxPenalties >= 4, nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
