package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
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
			return nil, err
		}
		err = os.WriteFile("cfg.toml", exampledata, 0644)
		if err != nil {
			return nil, err
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
			return nil, fmt.Errorf("Could not connect to %s", server.Address)
		}
	}

	return &conf, nil
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
	telegramService, err := telegram.New(notifyCfg.Api)
	if err != nil {
		log.Fatal(err)
	}

	telegramService.AddReceivers(notifyCfg.Chat)
	notify.UseServices(telegramService)

}
