// Perintah: go run ./cmd/migrate up | down [N]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"workshop-be/config"
	"workshop-be/infra/database"
	"workshop-be/infra/database/migrations"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate up | down [steps]")
		os.Exit(2)
	}
	cfg := config.Load()
	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DB.URL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	switch os.Args[1] {
	case "up":
		ran, err := database.Up(ctx, db, migrations.FS)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("migrasi up selesai, diterapkan: %v\n", ran)
	case "down":
		steps := 1
		if len(os.Args) > 2 {
			if steps, err = strconv.Atoi(os.Args[2]); err != nil || steps < 1 {
				log.Fatal("steps harus bilangan bulat positif")
			}
		}
		ran, err := database.Down(ctx, db, migrations.FS, steps)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("migrasi down selesai, dibatalkan: %v\n", ran)
	default:
		fmt.Fprintln(os.Stderr, "perintah tidak dikenal:", os.Args[1])
		os.Exit(2)
	}
}
