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

// Endpoints returns the found endpoints from zeroconf.
func Endpoints(domain string, timeout time.Duration) ([]*wishlist.Endpoint, error) {
	log.Printf("getting %s from zeroconf on domain %q...", service, domain)
	r, err := zeroconf.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("zeroconf: could not create resolver: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	if err := r.Browse(ctx, service, domain, entries); err != nil {
		return nil, fmt.Errorf("zeroconf: could not browse services: %w", err)
	}

	endpoints := make([]*wishlist.Endpoint, 0, len(entries))
	for entry := range entries {
		hostname := strings.TrimSuffix(entry.HostName, ".")
		port := strconv.Itoa(entry.Port)
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    hostname,
			Address: net.JoinHostPort(hostname, port),
		})
	}
	return endpoints, nil
}
