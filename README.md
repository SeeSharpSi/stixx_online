# Qwixx Online

A web-based multiplayer implementation of the popular dice game Qwixx using Go, Templ, HTMX, and SQLite.

## Features

- **Multiplayer Support**: Create and join games with up to 4 players
- **Real-time Updates**: Game state updates automatically using HTMX polling
- **Persistent Storage**: Game state is stored in SQLite database
- **Responsive Design**: Works on desktop and mobile devices
- **Session Management**: Players can leave and rejoin games

## Prerequisites

- Go 1.23.3 or higher
- [Templ](https://templ.guide/) template engine

## Installation

1. Clone the repository:
```bash
git clone https://github.com/SeeSharpSi/stixx_online
cd stixx_online
```

2. Install dependencies:
```bash
go mod download
```

3. Generate templates:
```bash
templ generate
```

4. Build the application:
```bash
go build
```

## Running the Game

1. Start the server:
```bash
./stixx_online
```

By default, the server runs on `http://localhost:9779`

You can customize the port and address:
```bash
./stixx_online -port 8080 -address http://localhost
```

2. Open your browser and navigate to the server address

## How to Play Qwixx

### Game Setup
- 2-4 players
- 6 dice: 2 white dice and 4 colored dice (red, yellow, green, blue)
- Each player has a scoresheet with 4 colored rows:
  - Red and Yellow: Numbers 2-12 (ascending)
  - Green and Blue: Numbers 12-2 (descending)

### Game Rules

1. **On Your Turn**:
   - Roll all 6 dice
   - All players can mark the sum of the two white dice in any color row
   - The active player can also mark the sum of one white die + one colored die in the matching color row

2. **Marking Numbers**:
   - Numbers must be marked from left to right
   - You cannot mark a number to the left of an already marked number
   - Once a number is marked, it cannot be unmarked

3. **Locking Rows**:
   - To mark the rightmost number in a row (the lock), you must have at least 5 marks in that row
   - When a player marks the lock, that row is closed for all players
   - The game ends when 2 rows are locked

4. **Penalties**:
   - If the active player cannot or chooses not to mark any number using the colored dice, they take a penalty
   - Each penalty is worth -5 points
   - The game also ends if any player accumulates 4 penalties

### Scoring
- Points are awarded based on how many numbers are marked in each row:
  - 1 mark = 1 point
  - 2 marks = 3 points
  - 3 marks = 6 points
  - 4 marks = 10 points
  - 5 marks = 15 points
  - 6 marks = 21 points
  - 7 marks = 28 points
  - 8 marks = 36 points
  - 9 marks = 45 points
  - 10 marks = 55 points
  - 11 marks = 66 points
  - 12 marks = 78 points
- Subtract 5 points for each penalty
- The player with the highest total score wins!

## Game Flow

1. **Create or Join a Game**:
   - Click "Create New Game" to start a new game and get a 5-character game code
   - Click "Join Game" and enter a game code to join an existing game

2. **Game Lobby**:
   - Wait for other players to join
   - The game creator can start the game once at least 2 players have joined

3. **Playing the Game**:
   - The active player rolls the dice
   - All players can mark available numbers (highlighted in green)
   - Click on a number to mark it
   - The active player must either make a move or take a penalty to end their turn

4. **Game End**:
   - The game ends when 2 rows are locked or a player has 4 penalties
   - Final scores are displayed

## Technical Details

- **Backend**: Go with standard library HTTP server
- **Frontend**: Templ templates with HTMX for interactivity
- **Database**: SQLite for game state persistence
- **Styling**: Custom CSS with responsive design

## Project Structure

```
stixx_online/
├── server.go          # Main server and route handlers
├── db/
│   └── db.go         # Database models and operations
├── game/
│   └── qwixx.go      # Game logic and rules
├── templ/            # Templ templates
│   ├── index.templ   # Landing page
│   ├── lobby.templ   # Game lobby
│   └── game.templ    # Main game board
├── static/           # Static assets
│   ├── styles.css    # Custom styles
│   └── htmx.min.js   # HTMX library
└── qwixx.db         # SQLite database (created on first run)
```

## License

This project is provided as-is for educational and entertainment purposes.
