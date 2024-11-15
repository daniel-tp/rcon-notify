package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-co-op/gocron"
	"github.com/gorcon/rcon"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/telegram"
)

type Config struct {
	Server struct {
		Address  string `toml:"address"`
		Password string `toml:"password"`
		//Game     string   `toml:"game"`
		Ignore  []string `toml:"ignore"`
		Name    string   `toml:"name"`
		Seconds int      `toml:"seconds"`
	}
	Notify struct {
		Api    string `toml:"api"`
		Chat   int64  `toml:"chat"`
		Prefix string `toml:"prefix"`
	}
}

var alreadyOnline []string

func loadConfig() (*Config, error) {

	// Check if the config file exists and if it doesn't copy the example file
	if _, err := os.Stat("cfg.toml"); os.IsNotExist(err) {
		exampledata, err := os.ReadFile("cfg.toml.example")
		if err != nil {
			log.Fatal(err)
		}
		err = os.WriteFile("cfg.toml", exampledata, 0644)
		if err != nil {
			log.Fatal(err)
		}
		return nil, fmt.Errorf("cfg.toml not found, created new cfg.toml from example file, please fill in")
	}
	var conf Config
	_, err := toml.DecodeFile("cfg.toml", &conf)
	if err != nil {
		panic(err)
	}
	//TODO: Validate config options

	return &conf, nil
}

type PlayerCheckService struct {
	cfg       *Config
	scheduler *gocron.Scheduler
}

func NewPlayerCheckService(cfg *Config) *PlayerCheckService {
	scheduler := gocron.NewScheduler(time.UTC)
	playerCheckService := &PlayerCheckService{cfg: cfg, scheduler: scheduler}

	scheduler.Every(cfg.Server.Seconds).Seconds().Do(func() {
		log.Print("Checking for new players")

		newPlayers := checkPlayers(playerCheckService.cfg)

		if len(newPlayers) > 0 {
			log.Print("New Players: " + strings.Join(newPlayers, ", "))
			err := notify.Send(context.Background(),
				playerCheckService.cfg.Server.Name,
				playerCheckService.cfg.Notify.Prefix+" "+strings.Join(newPlayers, ", "))

			if err != nil {
				log.Fatal(err)
			}
		}

	})
	scheduler.StartAsync()
	return playerCheckService
}

func callServer(cfg *Config) string {
	conn, err := rcon.Dial(cfg.Server.Address, cfg.Server.Password)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	response, err := conn.Execute("/players o")
	if err != nil {
		log.Fatal(err)
	}
	return response
}

func checkPlayers(cfg *Config) []string {

	response := callServer(cfg)

	players := parsePlayers(response, cfg.Server.Ignore)

	if len(players) == 0 {
		return nil
	}

	newPlayers := filterNewPlayers(players)
	return newPlayers
}

func main() {
	log.Print("Loading Config")
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Completed Loading Config")

	log.Print("Loading Notifications")
	setupNotifications(cfg)
	log.Print("Completed Loading Notifications")

	log.Print("Starting Player Check Service")
	playerCheckService := NewPlayerCheckService(cfg)
	log.Print("Completed Starting Player Check Service")
	defer playerCheckService.scheduler.Stop()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Block until an interrupt signal is received
	<-stopChan

	// Perform graceful shutdown
	fmt.Println("Received interrupt signal, shutting down...")

}

func setupNotifications(cfg *Config) {
	telegramService, err := telegram.New(cfg.Notify.Api)
	if err != nil {
		log.Fatal(err)
	}

	telegramService.AddReceivers(cfg.Notify.Chat)
	notify.UseServices(telegramService)

}

func filterNewPlayers(players []string) []string {
	var newPlayers []string
	for _, player := range players {
		if !slices.Contains(alreadyOnline, player) {
			alreadyOnline = append(alreadyOnline, player)

			newPlayers = append(newPlayers, player)
		}
	}
	return newPlayers
}

func parsePlayers(response string, ignore []string) []string {
	lines := strings.Split(response, "\n")
	var players []string
	for index, line := range lines {
		if strings.HasPrefix(line, "Online players") {
			players = lines[index+1:]

			break
		}
	}
	var filteredPlayers []string
	for _, player := range players {
		if player == "" {
			continue
		}

		player = strings.TrimSuffix(player, " (online)")

		if !slices.Contains(ignore, player) {
			filteredPlayers = append(filteredPlayers, player)
		}
	}
	return filteredPlayers
}
