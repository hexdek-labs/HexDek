package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "data/hexdek.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	backupPath := dbPath + ".bak-pre-reset-" + time.Now().Format("20060102")
	data, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("read db for backup: %v", err)
	}
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		log.Fatalf("write backup: %v", err)
	}
	fmt.Printf("backed up to %s (%d bytes)\n", backupPath, len(data))

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var eloCount, gameCount, seatCount, cardCount int
	db.QueryRow("SELECT COUNT(*) FROM showmatch_elo").Scan(&eloCount)
	db.QueryRow("SELECT COUNT(*) FROM showmatch_game").Scan(&gameCount)
	db.QueryRow("SELECT COUNT(*) FROM showmatch_game_seat").Scan(&seatCount)

	hasCardStats := true
	if err := db.QueryRow("SELECT COUNT(*) FROM showmatch_card_stats").Scan(&cardCount); err != nil {
		hasCardStats = false
	}

	fmt.Printf("before reset: %d elo entries, %d games, %d seats", eloCount, gameCount, seatCount)
	if hasCardStats {
		fmt.Printf(", %d card stats", cardCount)
	}
	fmt.Println()

	db.Exec("DELETE FROM showmatch_elo")
	db.Exec("DELETE FROM showmatch_game_seat")
	db.Exec("DELETE FROM showmatch_game")
	if hasCardStats {
		db.Exec("DELETE FROM showmatch_card_stats")
	}

	fmt.Println("ELO reset complete — all rating tables cleared")
}
