package main

import (
	"log"

	"github.com/gorcon/rcon"
)

func checkServer(cfg *Server) bool {
	conn, err := rcon.Dial(cfg.Address, cfg.Password)
	if err != nil {
		log.Println(err)
		return false
	}
	defer conn.Close()
	return true
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
