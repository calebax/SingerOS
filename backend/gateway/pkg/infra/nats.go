// Package infra provides infrastructure adapters for the gateway.
//
// Currently it contains the NATS event publisher that bridges the
// gateway's MessageEnvelope to the main project's event bus.
package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// NatsPublisher implements dispatch.EventPublisher by publishing
// normalized MessageEnvelope events to NATS topics.
//
// Topic format: interaction.{channel}.{message_type}
//   Example: interaction.qqbot.text, interaction.github.pull_request
//
// This format is compatible with the main project's eventengine.Orchestrator.
type NatsPublisher struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewNatsPublisher creates a NATS publisher connected to the given URL.
func NewNatsPublisher(url string) (*NatsPublisher, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS at %s: %w", url, err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	return &NatsPublisher{conn: conn, js: js}, nil
}

// Publish serializes the event as JSON and publishes it to the
// interaction topic for the channel.
func (p *NatsPublisher) Publish(ctx context.Context, topic string, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = p.js.Publish(topic, body, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish to %s: %w", topic, err)
	}

	return nil
}

// Close disconnects from NATS.
func (p *NatsPublisher) Close() {
	p.conn.Close()
}
