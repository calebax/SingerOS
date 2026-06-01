package whatsapp

import "time"

// Bridge API endpoints on the Node bridge.
const (
	bridgeHealthPath   = "/health"
	bridgeMessagesPath = "/messages"
	bridgeSendPath     = "/send"
	bridgeSendMediaPath = "/send-media"
)

// Timeouts.
const (
	bridgeStartTimeout  = 30 * time.Second
	bridgeHealthTimeout = 5 * time.Second
	bridgeAPITimeout    = 15 * time.Second
	bridgePollInterval  = 1 * time.Second
)
