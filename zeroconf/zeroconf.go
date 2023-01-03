package zeroconf

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/wishlist"
	"github.com/grandcat/zeroconf"
)

const service = "_ssh._tcp"

var timeout = 5 * time.Second

// Endpoints returns the found endpoints from zeroconf.
func Endpoints() ([]*wishlist.Endpoint, error) {
	log.Printf("getting %s from zeroconf...", service)
	r, err := zeroconf.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("zeroconf: could not create resolver: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	if err := r.Browse(ctx, service, "", entries); err != nil {
		return nil, fmt.Errorf("zeroconf: could not browse services: %w", err)
	}

	endpoints := make([]*wishlist.Endpoint, 0, len(entries))
	for entry := range entries {
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    strings.TrimSuffix(entry.HostName, "."),
			Address: net.JoinHostPort(entry.HostName, strconv.Itoa(entry.Port)),
		})
	}
	return endpoints, nil
}
