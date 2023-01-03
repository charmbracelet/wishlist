package zeroconf

import (
	"context"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/wishlist"
	"github.com/grandcat/zeroconf"
)

const service = "_ssh._tcp"

// Endpoints returns the found endpoints from zeroconf.
func Endpoints() ([]*wishlist.Endpoint, error) {
	log.Printf("getting %s from zeroconf...", service)
	r, err := zeroconf.NewResolver()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	if err := r.Browse(ctx, service, "", entries); err != nil {
		return nil, err
	}

	var endpoints []*wishlist.Endpoint
	for entry := range entries {
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    strings.TrimSuffix(entry.HostName, "."),
			Address: net.JoinHostPort(entry.HostName, strconv.Itoa(entry.Port)),
		})
	}
	return endpoints, nil
}
