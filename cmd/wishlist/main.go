package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/accesscontrol"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/gliderlabs/ssh"
)

type Config struct {
	Apps []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
}

func main() {
	listen := flag.String("listen", "127.0.0.1", "address to listen on")
	port := flag.Int64("port", 2222, "port to listen on")
	file := flag.String("config", "wishlist.json", "path to config file")
	flag.Parse()

	bts, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalln(err)
	}

	var config Config
	if err := json.Unmarshal(bts, &config); err != nil {
		log.Fatalln(err)
	}

	wc := wishlist.Config{
		Listen: *listen,
		Port:   *port,
		Factory: func(e wishlist.Endpoint) (*ssh.Server, error) {
			return wish.NewServer(
				wish.WithAddress(e.Address),
				wish.WithMiddleware(
					bm.Middleware(e.Handler),
					lm.Middleware(),
					accesscontrol.Middleware(),
					activeterm.Middleware(),
				),
			)
		},
	}
	for _, app := range config.Apps {
		wc.Endpoints = append(wc.Endpoints, &wishlist.Endpoint{
			Name:    app.Name,
			Address: app.Address,
		})
	}

	if err := wishlist.List(&wc); err != nil {
		log.Fatalln(err)
	}
}
