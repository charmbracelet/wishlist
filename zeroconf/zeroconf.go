package zeroconf

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wishlist"
	"github.com/grandcat/zeroconf"
)

const service = "_ssh._tcp"

// Endpoints returns the found endpoints from zeroconf.
func Endpoints(ctx context.Context, domain string, timeout time.Duration) ([]*wishlist.Endpoint, error) {
	log.Debug("discovering from zeroconf", "service", service, "domain", domain)
	r, err := zeroconf.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("zeroconf: could not create resolver: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
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
	log.Info("discovered from zeroconf", "service", service, "domain", domain, "devices", len(endpoints))
	return endpoints, nil
}
