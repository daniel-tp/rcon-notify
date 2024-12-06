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

type (
	Config struct {
		Servers map[string]Server
		Notify  Notify
	}

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
)

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
	//TODO: Deep Validating config (Servers, Notifications, etc)
	log.Println("Verifying Config")
	for _, server := range conf.Servers {
		log.Println("Verifying Server: \"" + server.Name + "\"")
		if !checkServer(&server) {
			log.Fatal("Could not connect to " + server.Address + " for " + server.Name)
			return nil, fmt.Errorf("Could not connect to " + server.Address)
		}
	}

	return &conf, nil
}

func checkServer(cfg *Server) bool {
	conn, err := rcon.Dial(cfg.Address, cfg.Password)
	if err != nil {
		log.Println(err)
		return false
	}
	defer conn.Close()
	return true
}

type PlayerCheckService struct {
	serverCfg *Server
	notifyCfg *Notify
	scheduler *gocron.Scheduler
}

func NewPlayerCheckService(serverCfg *Server, notifyCfg *Notify) *PlayerCheckService {
	scheduler := gocron.NewScheduler(time.UTC)
	playerCheckService := &PlayerCheckService{serverCfg: serverCfg, notifyCfg: notifyCfg, scheduler: scheduler}

	scheduler.Every(serverCfg.Seconds).Seconds().Do(func() {
		log.Print("Checking for new players")

		newPlayers := checkPlayers(playerCheckService.serverCfg)

		if len(newPlayers) > 0 {
			log.Print("New Players: " + strings.Join(newPlayers, ", "))
			err := notify.Send(context.Background(),
				playerCheckService.serverCfg.Name,
				playerCheckService.notifyCfg.Prefix+" "+strings.Join(newPlayers, ", "))

			if err != nil {
				log.Fatal(err)
			}
		}

	})
	scheduler.StartAsync()
	return playerCheckService
}

func callServer(cfg *Server) string {
	conn, err := rcon.Dial(cfg.Address, cfg.Password)
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

func checkPlayers(cfg *Server) []string {

	response := callServer(cfg)

	players := parsePlayers(response, cfg.Ignore)

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
	setupNotifications(&cfg.Notify)
	log.Print("Completed Loading Notifications")

	for _, server := range cfg.Servers {
		log.Print("Starting Player Check Service")
		playerCheckService := NewPlayerCheckService(&server, &cfg.Notify)
		log.Print("Completed Starting Player Check Service")
		defer playerCheckService.scheduler.Stop()
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Block until an interrupt signal is received
	<-stopChan

	// Perform graceful shutdown
	fmt.Println("Received interrupt signal, shutting down...")

}

func setupNotifications(notifyCfg *Notify) {
	log.Println("Joining chat: " + string(notifyCfg.Chat))
	telegramService, err := telegram.New(notifyCfg.Api)
	if err != nil {
		log.Fatal(err)
	}

	telegramService.AddReceivers(notifyCfg.Chat)
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
