package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: pds-discord-auth <command>")
		fmt.Println("Commands:")
		fmt.Println("  run     - Start the Discord bot")
		fmt.Println("  adduser - Add or update a user in the database")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "run":
		runBot()
	case "adduser":
		addUser()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runBot() {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN environment variable is required")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		log.Fatal("DATABASE_PATH environment variable is required")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout = 5000;
		PRAGMA synchronous = NORMAL;
		PRAGMA cache_size = 1000000000;
		PRAGMA foreign_keys = true;
		PRAGMA temp_store = memory;
	`)
	if err != nil {
		log.Fatal("Error setting database pragmas:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS "users" (
			did TEXT PRIMARY KEY,
			discord_user_id BIGINT
		);
	`)
	if err != nil {
		log.Fatal("Error creating users table:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS "sent_tokens" (
			token TEXT PRIMARY KEY
		);
	`)
	if err != nil {
		log.Fatal("Error creating sent_tokens table:", err)
	}

	pdsAccountPath := os.Getenv("PDS_ACCOUNT_SQLITE_PATH")
	if pdsAccountPath == "" {
		log.Fatal("PDS_ACCOUNT_SQLITE_PATH environment variable is required")
	}

	pdsAccountDB, err := sql.Open("sqlite3", "file:"+pdsAccountPath+"?mode=ro")
	if err != nil {
		log.Fatal("Error opening PDS account database:", err)
	}
	defer pdsAccountDB.Close()

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("Error creating Discord session:", err)
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening connection:", err)
	}

	go pollEmailTokens(pdsAccountDB, db, dg)

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func pollEmailTokens(pdsAccountDB *sql.DB, botDB *sql.DB, dg *discordgo.Session) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rows, err := pdsAccountDB.Query(`
			SELECT purpose, did, token, requestedAt
			FROM email_token
		`)
		if err != nil {
			log.Printf("Error querying email_token table: %v", err)
			continue
		}

		for rows.Next() {
			var purpose, did, token, requestedAt string
			err := rows.Scan(&purpose, &did, &token, &requestedAt)
			if err != nil {
				log.Printf("Error scanning email_token row: %v", err)
				continue
			}

			var exists int
			err = botDB.QueryRow("SELECT COUNT(*) FROM sent_tokens WHERE token = ?", token).Scan(&exists)
			if err != nil {
				log.Printf("Error checking sent_tokens: %v", err)
				continue
			}

			if exists > 0 {
				continue
			}

			var discordUserID int64
			err = botDB.QueryRow("SELECT discord_user_id FROM users WHERE did = ?", did).Scan(&discordUserID)
			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("No Discord user found for DID: %s", did)
				} else {
					log.Printf("Error looking up Discord user: %v", err)
				}
				continue
			}

			channel, err := dg.UserChannelCreate(strconv.FormatInt(discordUserID, 10))
			if err != nil {
				log.Printf("Error creating DM channel for user %d: %v", discordUserID, err)
				continue
			}

			_, err = dg.ChannelMessageSend(channel.ID, fmt.Sprintf("Your verification token: %s", token))
			if err != nil {
				log.Printf("Error sending DM to user %d: %v", discordUserID, err)
				continue
			}

			_, err = botDB.Exec("INSERT INTO sent_tokens (token) VALUES (?)", token)
			if err != nil {
				log.Printf("Error inserting into sent_tokens: %v", err)
			} else {
				fmt.Printf("Sent token %s to Discord user %d for DID %s\n", token, discordUserID, did)
			}
		}
		rows.Close()
	}
}

func addUser() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pds-discord-auth adduser <did> <discord_user_id>")
		os.Exit(1)
	}

	did := os.Args[2]
	discordUserIDStr := os.Args[3]

	discordUserID, err := strconv.ParseInt(discordUserIDStr, 10, 64)
	if err != nil {
		log.Fatal("Invalid discord_user_id: must be a number")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		log.Fatal("DATABASE_PATH environment variable is required")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO users (did, discord_user_id) VALUES (?, ?)
		ON CONFLICT(did) DO UPDATE SET discord_user_id = excluded.discord_user_id
	`, did, discordUserID)
	if err != nil {
		log.Fatal("Error upserting user:", err)
	}

	fmt.Printf("Successfully added/updated user: did=%s, discord_user_id=%d\n", did, discordUserID)
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	fmt.Printf("Message received - Channel: %s, Author: %s, Content: %s\n",
		m.ChannelID, m.Author.Username, m.Content)
}