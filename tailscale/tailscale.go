package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wishlist"
	"golang.org/x/oauth2/clientcredentials"
)

// Endpoints returns the found endpoints from tailscale.
func Endpoints(ctx context.Context, tailnet, key, clientID, clientSecret string) ([]*wishlist.Endpoint, error) {
	log.Debug("discovering from tailscale", "tailnet", tailnet)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", tailnet),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("tailscale: %w", err)
	}

	cli, err := getClient(key, clientID, clientSecret)
	if err != nil {
		return nil, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tailscale: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tailscale: %w", err)
	}

	if resp.StatusCode != 200 {
		var out nonOK
		if err := json.Unmarshal(bts, &out); err != nil {
			return nil, fmt.Errorf("tailscale: %w", err)
		}
		return nil, fmt.Errorf("tailscale: %s", out.Message)
	}

	var devices devices
	if err := json.Unmarshal(bts, &devices); err != nil {
		return nil, fmt.Errorf("tailscale: %w", err)
	}

	endpoints := make([]*wishlist.Endpoint, 0, len(devices.Devices))
	for _, device := range devices.Devices {
		endpoints = append(endpoints, &wishlist.Endpoint{
			Name:    device.Hostname,
			Address: net.JoinHostPort(device.Addresses[0], "22"),
		})
	}

	log.Info("discovered from tailscale", "tailnet", tailnet, "devices", len(endpoints))
	return endpoints, nil
}

func getClient(key, clientID, clientSecret string) (*http.Client, error) {
	if clientID != "" && clientSecret != "" {
		log.Info("using oauth")
		oauthConfig := &clientcredentials.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TokenURL:     "https://api.tailscale.com/api/v2/oauth/token",
		}
		return oauthConfig.Client(context.Background()), nil
	}

	if key != "" {
		log.Info("using api key auth")
		return &http.Client{
			Transport: apiKeyRoundTripper{key},
		}, nil
	}

	return nil, fmt.Errorf("tailscale: missing key or client configuration")
}

type apiKeyRoundTripper struct {
	key string
}

func (r apiKeyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.key))
	return http.DefaultTransport.RoundTrip(req)
}

type nonOK struct {
	Message string `json:"message"`
}

type devices struct {
	Devices []device `json:"devices"`
}

type device struct {
	ID         string   `json:"id"`
	Addresses  []string `json:"addresses"`
	Authorized bool     `json:"Authorized"`
	Hostname   string   `json:"hostname"`
}
