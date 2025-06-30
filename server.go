package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"seesharpsi/stixx_online/db"
	"seesharpsi/stixx_online/game"
	"seesharpsi/stixx_online/templ"
)

// Session management
type Session struct {
	PlayerID int
	GameCode string
}

var (
	sessions = make(map[string]*Session)
	mu       sync.RWMutex
)

func main() {
	port := flag.Int("port", 9779, "port the server runs on")
	address := flag.String("address", "http://localhost", "address the server runs on")
	flag.Parse()

	// Initialize database
	err := db.InitDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// ip parsing
	base_ip := *address
	ip := base_ip + ":" + strconv.Itoa(*port)
	root_ip, err := url.Parse(ip)
	if err != nil {
		log.Panic(err)
	}

	mux := http.NewServeMux()
	add_routes(mux)

	server := http.Server{
		Addr:    root_ip.Host,
		Handler: mux,
	}

	// start server
	log.Printf("running server on %s\n", root_ip.Host)
	err = server.ListenAndServe()
	defer server.Close()
	if errors.Is(err, http.ErrServerClosed) {
		log.Printf("server closed\n")
	} else if err != nil {
		log.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

func add_routes(mux *http.ServeMux) {
	mux.HandleFunc("/", GetIndex)
	mux.HandleFunc("/static/{file}", ServeStatic)
	mux.HandleFunc("/test", GetTest)

	// Game routes
	mux.HandleFunc("POST /create-game", CreateGame)
	mux.HandleFunc("POST /join-game", JoinGame)
	mux.HandleFunc("GET /lobby/{gameCode}", GetLobby)
	mux.HandleFunc("POST /start-game/{gameCode}", StartGame)
	mux.HandleFunc("GET /game/{gameCode}", GetGame)
	mux.HandleFunc("POST /roll-dice/{gameCode}", RollDice)
	mux.HandleFunc("POST /make-move", MakeMove)
	mux.HandleFunc("POST /end-turn/{gameCode}", EndTurn)
	mux.HandleFunc("POST /leave-game", LeaveGame)
}

func ServeStatic(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	log.Printf("got /static/%s request\n", file)
	http.ServeFile(w, r, "./static/"+file)
}

func GetIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("got / request\n")
	component := templ.Index()
	component.Render(context.Background(), w)
}

func GetTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("got /test request\n")
	component := templ.Test()
	component.Render(context.Background(), w)
}

func CreateGame(w http.ResponseWriter, r *http.Request) {
	log.Printf("got /create-game request\n")

	err := r.ParseForm()
	if err != nil {
		w.Write([]byte(`<div class="error">Invalid form data</div>`))
		return
	}

	name := r.FormValue("name")
	if name == "" {
		w.Write([]byte(`<div class="error">Please enter your name</div>`))
		return
	}

	// Create game
	game, err := db.CreateGame()
	if err != nil {
		w.Write([]byte(`<div class="error">Failed to create game</div>`))
		return
	}

	// Join as first player
	player, err := db.JoinGame(game.GameCode, name)
	if err != nil {
		w.Write([]byte(`<div class="error">Failed to join game</div>`))
		return
	}

	// Create session
	sessionID := generateSessionID()
	mu.Lock()
	sessions[sessionID] = &Session{
		PlayerID: player.ID,
		GameCode: game.GameCode,
	}
	mu.Unlock()

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
	})

	// Redirect to lobby
	w.Header().Set("HX-Redirect", fmt.Sprintf("/lobby/%s", game.GameCode))
	w.Write([]byte(`<div class="success">Game created! Redirecting...</div>`))
}

func JoinGame(w http.ResponseWriter, r *http.Request) {
	log.Printf("got /join-game request\n")

	err := r.ParseForm()
	if err != nil {
		w.Write([]byte(`<div class="error">Invalid form data</div>`))
		return
	}

	name := r.FormValue("name")
	gameCode := r.FormValue("gameCode")

	if name == "" || gameCode == "" {
		w.Write([]byte(`<div class="error">Please fill in all fields</div>`))
		return
	}

	// Join game
	player, err := db.JoinGame(gameCode, name)
	if err != nil {
		w.Write([]byte(`<div class="error">Failed to join game. Check the game code and try again.</div>`))
		return
	}

	// Create session
	sessionID := generateSessionID()
	mu.Lock()
	sessions[sessionID] = &Session{
		PlayerID: player.ID,
		GameCode: gameCode,
	}
	mu.Unlock()

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
	})

	// Redirect to lobby
	w.Header().Set("HX-Redirect", fmt.Sprintf("/lobby/%s", gameCode))
	w.Write([]byte(`<div class="success">Joined game! Redirecting...</div>`))
}

func GetLobby(w http.ResponseWriter, r *http.Request) {
	gameCode := r.PathValue("gameCode")
	log.Printf("got /lobby/%s request\n", gameCode)

	// Get session
	session := getSession(r)
	if session == nil || session.GameCode != gameCode {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get game
	game, err := db.GetGame(gameCode)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// If game has started, redirect to game
	if game.Status == "active" || game.Status == "finished" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/game/%s", gameCode))
		return
	}

	// Get players
	players, err := db.GetPlayers(game.ID)
	if err != nil {
		http.Error(w, "Failed to get players", http.StatusInternalServerError)
		return
	}

	// Check if current player is creator
	isCreator := false
	if len(players) > 0 && players[0].ID == session.PlayerID {
		isCreator = true
	}

	component := templ.Lobby(game, players, session.PlayerID, isCreator)
	component.Render(context.Background(), w)
}

func StartGame(w http.ResponseWriter, r *http.Request) {
	gameCode := r.PathValue("gameCode")
	log.Printf("got /start-game/%s request\n", gameCode)

	// Get session
	session := getSession(r)
	if session == nil || session.GameCode != gameCode {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get game
	game, err := db.GetGame(gameCode)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Get players
	players, err := db.GetPlayers(game.ID)
	if err != nil {
		http.Error(w, "Failed to get players", http.StatusInternalServerError)
		return
	}

	// Verify current player is creator
	if len(players) == 0 || players[0].ID != session.PlayerID {
		http.Error(w, "Only the game creator can start the game", http.StatusUnauthorized)
		return
	}

	// Start game
	err = db.StartGame(game.ID)
	if err != nil {
		http.Error(w, "Failed to start game", http.StatusInternalServerError)
		return
	}

	// Roll initial dice
	err = db.RollDice(game.ID)
	if err != nil {
		http.Error(w, "Failed to roll dice", http.StatusInternalServerError)
		return
	}

	// Redirect to game
	w.Header().Set("HX-Redirect", fmt.Sprintf("/game/%s", gameCode))
}

func GetGame(w http.ResponseWriter, r *http.Request) {
	gameCode := r.PathValue("gameCode")
	log.Printf("got /game/%s request\n", gameCode)

	// Get session
	session := getSession(r)
	if session == nil || session.GameCode != gameCode {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Load game state
	gameState, err := game.LoadGameState(gameCode)
	if err != nil {
		http.Error(w, "Failed to load game", http.StatusInternalServerError)
		return
	}

	// Get current player
	var currentPlayer *db.Player
	for _, p := range gameState.Players {
		if p.ID == session.PlayerID {
			currentPlayer = &p
			break
		}
	}

	if currentPlayer == nil {
		http.Error(w, "Player not found in game", http.StatusBadRequest)
		return
	}

	// Check if it's the current player's turn
	isCurrentPlayerTurn := gameState.Players[gameState.Game.CurrentPlayerIndex].ID == session.PlayerID

	// Get possible moves
	possibleMoves, err := game.GetPossibleMoves(session.PlayerID, gameState.Game, isCurrentPlayerTurn)
	if err != nil {
		http.Error(w, "Failed to get possible moves", http.StatusInternalServerError)
		return
	}

	// Get all player marks
	playerMarks := make(map[int]map[string][]int)
	scores := make(map[int]int)
	for _, player := range gameState.Players {
		marks, err := game.GetPlayerMarkedNumbers(player.ID)
		if err == nil {
			playerMarks[player.ID] = marks
		}

		score, err := game.CalculateScore(player.ID)
		if err == nil {
			scores[player.ID] = score
		}
	}

	component := templ.Game(gameState, session.PlayerID, possibleMoves, playerMarks, scores)
	component.Render(context.Background(), w)
}

func RollDice(w http.ResponseWriter, r *http.Request) {
	gameCode := r.PathValue("gameCode")
	log.Printf("got /roll-dice/%s request\n", gameCode)

	// Get session
	session := getSession(r)
	if session == nil || session.GameCode != gameCode {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get game
	gameData, err := db.GetGame(gameCode)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify it's the current player's turn
	currentPlayer, err := game.GetCurrentPlayer(gameData.ID)
	if err != nil || currentPlayer.ID != session.PlayerID {
		http.Error(w, "Not your turn", http.StatusBadRequest)
		return
	}

	// Check if dice already rolled this turn
	if gameData.DiceRolled {
		http.Error(w, "Dice already rolled this turn", http.StatusBadRequest)
		return
	}

	// Roll dice
	err = db.RollDice(gameData.ID)
	if err != nil {
		http.Error(w, "Failed to roll dice", http.StatusInternalServerError)
		return
	}

	// Redirect back to game
	w.Header().Set("HX-Redirect", fmt.Sprintf("/game/%s", gameCode))
}

func MakeMove(w http.ResponseWriter, r *http.Request) {
	log.Printf("got /make-move request\n")

	// Get session
	session := getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form values
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	color := r.FormValue("color")
	numberStr := r.FormValue("number")
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		http.Error(w, "Invalid number", http.StatusBadRequest)
		return
	}

	// Get game with all fields
	var gameData db.Game
	err = db.DB.QueryRow(`
		SELECT id, game_code, status, created_at, current_player_index,
		       white_dice_1, white_dice_2, red_dice, yellow_dice, green_dice, blue_dice,
		       red_locked, yellow_locked, green_locked, blue_locked, penalties_triggered,
		       dice_rolled, white_mark_used, colored_mark_used
		FROM games WHERE game_code = ?
	`, session.GameCode).Scan(
		&gameData.ID, &gameData.GameCode, &gameData.Status, &gameData.CreatedAt, &gameData.CurrentPlayerIndex,
		&gameData.WhiteDice1, &gameData.WhiteDice2, &gameData.RedDice, &gameData.YellowDice, &gameData.GreenDice, &gameData.BlueDice,
		&gameData.RedLocked, &gameData.YellowLocked, &gameData.GreenLocked, &gameData.BlueLocked, &gameData.PenaltiesTriggered,
		&gameData.DiceRolled, &gameData.WhiteMarkUsed, &gameData.ColoredMarkUsed,
	)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Determine move type
	moveType := "white"
	whiteSum := gameData.WhiteDice1 + gameData.WhiteDice2

	// Check if this is a colored move (for active player)
	currentPlayer, err := game.GetCurrentPlayer(gameData.ID)
	if err == nil && currentPlayer.ID == session.PlayerID {
		// Check if it's a colored dice combination
		colorDice := map[string]int{
			"red":    gameData.RedDice,
			"yellow": gameData.YellowDice,
			"green":  gameData.GreenDice,
			"blue":   gameData.BlueDice,
		}

		if colorValue, ok := colorDice[color]; ok {
			if number == gameData.WhiteDice1+colorValue || number == gameData.WhiteDice2+colorValue {
				if number != whiteSum { // Not just the white sum
					moveType = "colored"
				}
			}
		}
	}

	// Make the mark
	err = game.MakeMark(session.PlayerID, color, number, gameData.ID, moveType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Redirect back to game
	w.Header().Set("HX-Redirect", fmt.Sprintf("/game/%s", session.GameCode))
}

func EndTurn(w http.ResponseWriter, r *http.Request) {
	gameCode := r.PathValue("gameCode")
	log.Printf("got /end-turn/%s request\n", gameCode)

	// Get session
	session := getSession(r)
	if session == nil || session.GameCode != gameCode {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get game
	gameData, err := db.GetGame(gameCode)
	if err != nil {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Verify it's the current player's turn
	currentPlayer, err := game.GetCurrentPlayer(gameData.ID)
	if err != nil || currentPlayer.ID != session.PlayerID {
		http.Error(w, "Not your turn", http.StatusBadRequest)
		return
	}

	// Add penalty
	err = db.AddPenalty(session.PlayerID)
	if err != nil {
		http.Error(w, "Failed to add penalty", http.StatusInternalServerError)
		return
	}

	// Process turn
	err = game.ProcessTurn(gameData.ID, make(map[int][]game.Move), true)
	if err != nil {
		http.Error(w, "Failed to process turn", http.StatusInternalServerError)
		return
	}

	// Don't roll dice yet - next player needs to do it themselves

	// Redirect back to game
	w.Header().Set("HX-Redirect", fmt.Sprintf("/game/%s", gameCode))
}

func LeaveGame(w http.ResponseWriter, r *http.Request) {
	log.Printf("got /leave-game request\n")

	// Clear session
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to home
	w.Header().Set("HX-Redirect", "/")
}

// Helper functions
func generateSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}

	mu.RLock()
	session := sessions[cookie.Value]
	mu.RUnlock()

	return session
}
