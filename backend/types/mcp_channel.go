package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

const (
	// MCPChannelStatusActive allows a channel to be displayed and used for new connections.
	MCPChannelStatusActive = "active"
	// MCPChannelStatusInactive keeps a channel configuration without allowing new connections.
	MCPChannelStatusInactive = "inactive"
)

// MCPChannelHeaders stores non-sensitive fixed HTTP headers for one MCP channel.
type MCPChannelHeaders map[string]string

// Scan implements sql.Scanner.
func (h *MCPChannelHeaders) Scan(value interface{}) error {
	if value == nil {
		*h = MCPChannelHeaders{}
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("cannot scan %T into MCPChannelHeaders", value)
	}
	result := make(map[string]string)
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*h = result
	return nil
}

// Value implements driver.Valuer and always stores a JSON object.
func (h MCPChannelHeaders) Value() (driver.Value, error) {
	if len(h) == 0 {
		return "{}", nil
	}
	return json.Marshal(map[string]string(h))
}

// MCPChannel is a system-maintained template for one built-in MCP platform.
type MCPChannel struct {
	gorm.Model

	Channel     string            `gorm:"column:channel;type:varchar(64);not null;uniqueIndex:ux_mcp_channel_channel"`
	Name        string            `gorm:"column:name;type:varchar(255);not null"`
	Description string            `gorm:"column:description;type:text"`
	Transport   string            `gorm:"column:transport;type:varchar(32);not null"`
	URL         string            `gorm:"column:url;type:varchar(2000);not null"`
	Headers     MCPChannelHeaders `gorm:"column:headers;type:jsonb;not null;default:'{}'"`
	Status      string            `gorm:"column:status;type:varchar(32);not null;default:'active';index"`
}

// TableName returns the MCP channel configuration table name.
func (MCPChannel) TableName() string {
	return TableNameMCPChannel
}
