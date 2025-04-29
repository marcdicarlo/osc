/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	// "fmt"
	"log"

	"github.com/marcdicarlo/osc/cmd"
	"github.com/marcdicarlo/osc/internal/config"
	"github.com/marcdicarlo/osc/internal/db"
)

func main() {
	// Load configuration from YAML
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cmd.Execute()

	// Print the loaded configuration
	// fmt.Printf("Loaded config: %+v\n", cfg)

	// initalize the database
	database, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	defer database.Close()

}
