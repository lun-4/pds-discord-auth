package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
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

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	fmt.Printf("Message received - Channel: %s, Author: %s, Content: %s\n",
		m.ChannelID, m.Author.Username, m.Content)
}