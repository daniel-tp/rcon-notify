package main

import (
	"context"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/nikoksr/notify"
)

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

func checkPlayers(cfg *Server) []string {

	response := callServer(cfg)

	players := parsePlayers(response, cfg.Ignore)

	if len(players) == 0 {
		return nil
	}

	newPlayers := filterNewPlayers(players)
	return newPlayers
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
