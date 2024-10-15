package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID       int64
	Name     string
	Stick    int
	LastGrow time.Time
	ChatID   int64
}

var db *sql.DB
var users = make(map[int64]map[int64]*User) // chatID -> map of userID -> User
var botToken = "ff"

func main() {
	er := godotenv.Load()
	if er != nil {
		log.Fatal("Error loading .env file")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN not set in .env file")
	}

	var err error
	db, err = sql.Open("sqlite3", "./dicks.db")
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

	go checkDailyReminder(bot)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID
		userName := update.Message.From.UserName

		// welcome message
		if len(update.Message.NewChatMembers) > 0 {
			for _, user := range update.Message.NewChatMembers {
				if user.UserName == bot.Self.UserName {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMessage())
					bot.Send(msg)
				}
			}
		}

		if users[chatID] == nil {
			users[chatID] = make(map[int64]*User)
		}

		user, exists := users[chatID][userID]
		if !exists {
			user = loadUser(userID, userName, chatID)
			users[chatID][userID] = user
		}

		switch update.Message.Command() {
		case "grow":
			response := growStick(user)
			saveUser(user)
			msg := tgbotapi.NewMessage(chatID, response)
			bot.Send(msg)
		case "leaderboard":
			msg := tgbotapi.NewMessage(chatID, showLeaderboard(chatID))
			bot.Send(msg)
		case "dicks":
			msg := tgbotapi.NewMessage(chatID, showGraphicalSticks(chatID))
			bot.Send(msg)
		case "stickerpack":
			msg := tgbotapi.NewMessage(chatID, "Check out the sticker pack: https://t.me/addstickers/traspppppc")
			bot.Send(msg)
		}
	}
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER,
		name TEXT,
		stick INTEGER,
		last_grow TIMESTAMP,
		chat_id INTEGER,
		PRIMARY KEY (id, chat_id)
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func loadUser(userID int64, userName string, chatID int64) *User {
	var stick int
	var lastGrow time.Time

	query := `SELECT stick, last_grow FROM users WHERE id = ? AND chat_id = ?`
	row := db.QueryRow(query, userID, chatID)
	err := row.Scan(&stick, &lastGrow)

	if err == sql.ErrNoRows {
		// if no user found create one
		user := &User{ID: userID, Name: userName, Stick: 0, LastGrow: time.Now().Add(-24 * time.Hour), ChatID: chatID}
		saveUser(user)
		return user
	} else if err != nil {
		log.Printf("Failed to load user %v: %v", userID, err)
		return nil
	}

	return &User{ID: userID, Name: userName, Stick: stick, LastGrow: lastGrow, ChatID: chatID}
}

func saveUser(user *User) {
	query := `
	INSERT INTO users (id, name, stick, last_grow, chat_id)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id, chat_id) DO UPDATE SET
	name = excluded.name,
	stick = excluded.stick,
	last_grow = excluded.last_grow;
	`
	_, err := db.Exec(query, user.ID, user.Name, user.Stick, user.LastGrow, user.ChatID)
	if err != nil {
		log.Printf("Failed to save user %v: %v", user.ID, err)
	}
}

func welcomeMessage() string {
	return `Добро пожаловать в Game of Dicks без рекламы и говна
Список доступных команд:
- /grow: Увеличивает или уменьшает твой член в диапазоне -10...+10
- /leaderboard: Показывает доску позора чата
- /dicks: Попытка в графическую репрезентацию
- /stickerpack: Посмотреть авторский стикер-пак
`
}

func growStick(user *User) string {
	now := time.Now()
	if now.Sub(user.LastGrow).Hours() < 24 {
		return fmt.Sprintf("Играть можно только раз в 24 часа. Попробуй позже.")
	}

	rand.Seed(time.Now().UnixNano())
	change := rand.Intn(21) - 10 // random value between -10 and +10
	user.Stick += change

	// not negative
	if user.Stick < 0 {
		user.Stick = 0
	}

	user.LastGrow = now

	return fmt.Sprintf("%s's член изменился на %d cм! Текущая длина: %d см", user.Name, change, user.Stick)
}

func showLeaderboard(chatID int64) string {
	board := "Leaderboard:\n"
	type score struct {
		Name  string
		Stick int
	}
	var scores []score
	for _, user := range users[chatID] {
		scores = append(scores, score{Name: user.Name, Stick: user.Stick})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Stick > scores[j].Stick
	})

	for i, s := range scores {
		board += fmt.Sprintf("%d. %s: %d cm\n", i+1, s.Name, s.Stick)
	}

	return board
}

func showGraphicalSticks(chatID int64) string {
	graph := "График длин ваших членов:\n"
	type score struct {
		Name  string
		Stick int
	}
	var scores []score
	for _, user := range users[chatID] {
		scores = append(scores, score{Name: user.Name, Stick: user.Stick})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Stick > scores[j].Stick
	})

	maxLength := 0
	for _, s := range scores {
		if s.Stick > maxLength {
			maxLength = s.Stick
		}
	}

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

func checkDailyReminder(bot *tgbotapi.BotAPI) {
	for {
		time.Sleep(24 * time.Hour)
		now := time.Now()
		for chatID, userMap := range users {
			activity := false
			for _, user := range userMap {
				if now.Sub(user.LastGrow).Hours() < 24 {
					activity = true
					break
				}
			}
			if !activity {
				msg := tgbotapi.NewMessage(chatID, "Не забывайте о своем члене! Use /grow, /leaderboard, /dicks, or /stickerpack.")
				bot.Send(msg)
			}
		}
	}
}
