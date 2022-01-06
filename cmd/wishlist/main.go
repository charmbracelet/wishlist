package main

import (
	"flag"
	"log"
	"os"

	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/accesscontrol"
	"github.com/charmbracelet/wish/activeterm"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wishlist"
	"github.com/gliderlabs/ssh"
	"gopkg.in/yaml.v3"
)

func main() {
	file := flag.String("config", "wishlist.yaml", "path to config file")
	flag.Parse()

	bts, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalln(err)
	}

	var config wishlist.Config
	if err := yaml.Unmarshal(bts, &config); err != nil {
		log.Fatalln(err)
	}

	names := []string{"list"}
	for _, e := range config.Endpoints {
		names = append(names, e.Name)
	}

	config.Factory = func(e wishlist.Endpoint) (*ssh.Server, error) {
		return wish.NewServer(
			wish.WithAddress(e.Address),
			wish.WithMiddleware(
				append(
					e.Middlewares,
					lm.Middleware(),
					accesscontrol.Middleware(names...),
					activeterm.Middleware(),
				)...,
			),
		)
	}

	if err := wishlist.Serve(&config); err != nil {
		log.Fatalln(err)
	}
}
