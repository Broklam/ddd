package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID       int64
	Name     string
	Stick    int
	LastGrow time.Time
}

var db *sql.DB
var users = make(map[int64]*User)
var botToken = "8178907497:AAGg0T0-UpIuzQyXUkV3rkT8zzfCeE1qH9Y"

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./sticks.db")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	createTable()

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		userName := update.Message.From.UserName

		// Send a welcome message when the bot is added to a group
		if len(update.Message.NewChatMembers) > 0 {
			for _, user := range update.Message.NewChatMembers {
				if user.UserName == bot.Self.UserName {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMessage())
					bot.Send(msg)
				}
			}
		}

		user, exists := users[userID]
		if !exists {
			user = loadUser(userID, userName)
			users[userID] = user
		}

		switch update.Message.Command() {
		case "grow":
			response := growStick(user)
			saveUser(user)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
			bot.Send(msg)
		case "leaderboard":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, showLeaderboard())
			bot.Send(msg)
		case "sticks":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, showGraphicalSticks())
			bot.Send(msg)
		}
	}
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		name TEXT,
		stick INTEGER,
		last_grow TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func loadUser(userID int64, userName string) *User {
	var stick int
	var lastGrow time.Time

	query := `SELECT stick, last_grow FROM users WHERE id = ?`
	row := db.QueryRow(query, userID)
	err := row.Scan(&stick, &lastGrow)

	if err == sql.ErrNoRows {
		// if no user found create one
		user := &User{ID: userID, Name: userName, Stick: 0, LastGrow: time.Now().Add(-24 * time.Hour)}
		saveUser(user)
		return user
	} else if err != nil {
		log.Printf("Failed to load user %v: %v", userID, err)
		return nil
	}

	return &User{ID: userID, Name: userName, Stick: stick, LastGrow: lastGrow}
}

func saveUser(user *User) {
	query := `
	INSERT INTO users (id, name, stick, last_grow)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	stick = excluded.stick,
	last_grow = excluded.last_grow;
	`
	_, err := db.Exec(query, user.ID, user.Name, user.Stick, user.LastGrow)
	if err != nil {
		log.Printf("Failed to save user %v: %v", user.ID, err)
	}
}

func welcomeMessage() string {
	return `Добро пожаловать в Game of Dicks без рекламы и говна
Список доступных команд:
- /grow: Увеличивает или уменьшает твой член в диапазоне -10...+10
- /leaderboard: Показывает доску позора чата
- /sticks: Попытка в графическую репрезентацию
`
}

func growStick(user *User) string {
	now := time.Now()
	if now.Sub(user.LastGrow).Hours() < 24 {
		return fmt.Sprintf("Играть можно только раз в 24 часа. Попробуй позже.")
	}

	rand.Seed(time.Now().UnixNano())
	change := rand.Intn(21) - 10 // Random value between -10 and +10
	user.Stick += change
	user.LastGrow = now

	return fmt.Sprintf("%s's член изменился на %d cм! Текущая длина: %d см", user.Name, change, user.Stick)
}

func showLeaderboard() string {
	board := "Leaderboard:\n"
	type score struct {
		Name  string
		Stick int
	}
	var scores []score
	for _, user := range users {
		scores = append(scores, score{Name: user.Name, Stick: user.Stick})
	}

	// Sort by stick length (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Stick > scores[j].Stick
	})

	for i, s := range scores {
		board += fmt.Sprintf("%d. %s: %d cm\n", i+1, s.Name, s.Stick)
	}

	return board
}

func showGraphicalSticks() string {
	graph := "График членов кх кх:\n"
	type score struct {
		Name  string
		Stick int
	}
	var scores []score
	for _, user := range users {
		scores = append(scores, score{Name: user.Name, Stick: user.Stick})
	}

	// ужасная сортировка
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Stick > scores[j].Stick
	})

	// максимальная длина
	maxLength := 0
	for _, s := range scores {
		if s.Stick > maxLength {
			maxLength = s.Stick
		}
	}

	// попытка в визуал (говна)
	for _, s := range scores {
		bar := generateBar(s.Stick, maxLength)
		graph += fmt.Sprintf("%s: [%s] %d cm\n", s.Name, bar, s.Stick)
	}
	return graph
}

func generateBar(length, maxLength int) string {
	if maxLength == 0 {
		return ""
	}
	barLength := 20 // ширина палок
	unit := length * barLength / maxLength
	return strings.Repeat("█", unit)
}
