package srv

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wishlist"
)

const (
	service   = "_ssh._tcp"
	txtPrefix = "wishlist.name "
)

// Endpoints returns the _ssh._tcp SRV records on the given domain as
// Wishlist endpoints.
func Endpoints(ctx context.Context, domain string) ([]*wishlist.Endpoint, error) {
	log.Debug("discovering SRV records", "service", service, "domain", domain)
	_, srvs, err := net.DefaultResolver.LookupSRV(ctx, "ssh", "tcp", domain)
	if err != nil {
		return nil, fmt.Errorf("srv: could not resolve %s: %w", domain, err)
	}
	txts, err := net.DefaultResolver.LookupTXT(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("srv: could not resolve %s: %w", domain, err)
	}
	endpoints := fromRecords(srvs, txts)
	log.Info("discovered from SRV records", "service", service, "domain", domain, "devices", len(endpoints))
	return endpoints, nil
}

func fromRecords(srvs []*net.SRV, txts []string) []*wishlist.Endpoint {
	result := make([]*wishlist.Endpoint, 0, len(srvs))
	for _, entry := range srvs {
		hostname := strings.TrimSuffix(entry.Target, ".")
		name := hostname
		port := fmt.Sprintf("%d", entry.Port)
		address := net.JoinHostPort(hostname, port)
		for _, txt := range txts {
			if !strings.HasPrefix(txt, txtPrefix) {
				continue
			}
			txtAddr, txtName, ok := strings.Cut(strings.TrimPrefix(txt, txtPrefix), "=")
			if !ok {
				continue
			}
			if txtAddr == address {
				name = txtName
			}
		}
		result = append(result, &wishlist.Endpoint{
			Name:    name,
			Address: address,
		})
	}
	return result
}
