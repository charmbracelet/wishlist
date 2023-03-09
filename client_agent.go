package wishlist

import (
	"fmt"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func forwardAgent(agt agent.Agent, session *ssh.Session, client *ssh.Client) error {
	if agt == nil {
		log.Info("requested ForwardAgent, but no agent is available")
		return nil
	}
	if err := agent.RequestAgentForwarding(session); err != nil {
		return fmt.Errorf("failed to forward agent: %w", err)
	}
	if err := agent.ForwardToAgent(client, agt); err != nil {
		return fmt.Errorf("failed to forward agent: %w", err)
	}
	return nil
}
