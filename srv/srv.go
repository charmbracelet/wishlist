package srv

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/charmbracelet/wishlist"
)

// Endpoints returns the _ssh._tcp SRV records on the given domain as
// Wishlist endpoints.
func Endpoints(domain string) ([]*wishlist.Endpoint, error) {
	log.Printf("discovering _ssh._tcp SRV records on domain %q...", domain)
	_, srvs, err := net.LookupSRV("ssh", "tcp", domain)
	if err != nil {
		return nil, fmt.Errorf("srv: could not resolve %s: %w", domain, err)
	}
	result := make([]*wishlist.Endpoint, 0, len(srvs))
	for _, entry := range srvs {
		hostname := strings.TrimSuffix(entry.Target, ".")
		port := fmt.Sprintf("%d", entry.Port)
		result = append(result, &wishlist.Endpoint{
			Name:    hostname,
			Address: net.JoinHostPort(hostname, port),
		})
	}
	return result, nil
}
