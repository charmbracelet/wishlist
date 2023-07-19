package wishlist

import (
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	gossh "golang.org/x/crypto/ssh"
)

func proxyJump(addr, nextAddr string, conf, nextConf *gossh.ClientConfig) (*gossh.Client, closers, error) {
	var cl closers
	log.Info("connecting client to ProxyJump", "addr", addr)
	jumpClient, err := gossh.Dial("tcp", addr, conf)
	if err != nil {
		return nil, cl, fmt.Errorf("connection to ProxyJump (%s) failed: %w", addr, err)
	}
	cl = append(cl, jumpClient.Close)

	log.Info("connecting to target using jump client", "addr", nextAddr)
	jumpConn, err := jumpClient.Dial("tcp", nextAddr)
	if err != nil {
		return nil, cl, fmt.Errorf("connection from ProxyJump (%s) to Host (%s) failed: %w", addr, nextAddr, err)
	}
	cl = append(cl, jumpConn.Close)

	log.Info("getting client connection", "addr", nextAddr)
	ncc, chans, reqs, err := gossh.NewClientConn(jumpConn, nextAddr, nextConf)
	if err != nil {
		return nil, cl, fmt.Errorf("client connection from ProxyJump (%s) to Host (%s) failed: %w", addr, nextAddr, err)
	}
	cl = append(cl, ncc.Close)

	return gossh.NewClient(ncc, chans, reqs), cl, nil
}

func ensureJumpPort(addr string) string {
	if _, port, _ := net.SplitHostPort(addr); port == "" {
		return addr + ":22"
	}
	return addr
}

func splitJump(jump string) (string, string) {
	parts := strings.Split(jump, "@")
	switch len(parts) {
	case 1:
		return "", ensureJumpPort(parts[0]) // jump with no username
	case 2: //nolint: gomnd
		return parts[0], ensureJumpPort(parts[1]) // jump with user and host
	default:
		return strings.Join(parts[0:len(parts)-1], "@"), ensureJumpPort(parts[len(parts)-1])
	}
}
