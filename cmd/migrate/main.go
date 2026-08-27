package main

import (
	"errors"
	"flag"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"socialfund/internal/config"
)

func main() {
	direction := flag.String("direction", "up", "migration command: up, down, or version")
	steps := flag.Int("steps", 0, "number of migrations; zero means all for up/down")
	flag.Parse()
	databaseURL, err := config.LoadDatabaseURL()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			log.Printf("close migration source: %v", sourceErr)
		}
		if databaseErr != nil {
			log.Printf("close migration database: %v", databaseErr)
		}
	}()
	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	case "version":
		version, dirty, versionErr := m.Version()
		if versionErr != nil && !errors.Is(versionErr, migrate.ErrNilVersion) {
			log.Fatalf("read migration version: %v", versionErr)
		}
		if errors.Is(versionErr, migrate.ErrNilVersion) {
			log.Print("migration version: none")
			return
		}
		log.Printf("migration version: %d (dirty=%t)", version, dirty)
		return
	default:
		log.Fatalf("unsupported direction %q", *direction)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("run migrations: %v", err)
	}
}
