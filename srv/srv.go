package srv

import (
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wishlist"
)

// Endpoints returns the _ssh._tcp SRV records on the given domain as
// Wishlist endpoints.
func Endpoints(domain string) ([]*wishlist.Endpoint, error) {
	log.Info("discovering _ssh._tcp SRV records", "domain", domain)
	_, srvs, err := net.LookupSRV("ssh", "tcp", domain)
	if err != nil {
		return nil, fmt.Errorf("srv: could not resolve %s: %w", domain, err)
	}
	txts, err := net.LookupTXT(domain)
	if err != nil {
		return nil, fmt.Errorf("srv: could not resolve %s: %w", domain, err)
	}
	return fromRecords(srvs, txts), nil
}

const txtPrefix = "wishlist.name "

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
