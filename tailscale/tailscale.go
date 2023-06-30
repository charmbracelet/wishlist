package tailscale

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wishlist"
)

// Endpoints returns the found endpoints from tailscale.
func Endpoints(tailnet, key string) ([]*wishlist.Endpoint, error) {
	log.Info("discovering from tailscale", "tailnet", tailnet)
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", tailnet),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", key))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var devices struct {
		Devices []device `json:"devices"`
	}
	if err := json.Unmarshal(bts, &devices); err != nil {
		return nil, err
	}

	endpoints := make([]*wishlist.Endpoint, 0, len(devices.Devices))
	for _, device := range devices.Devices {
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    device.Hostname,
			Address: net.JoinHostPort(device.Addresses[0], "22"),
		})
	}
	return endpoints, nil
}

type device struct {
	ID         string   `json:"id"`
	Addresses  []string `json:"addresses"`
	Authorized bool     `json:"Authorized"`
	Hostname   string   `json:"hostname"`
}
