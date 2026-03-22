package examples

import (
	"time"

	"github.com/insmtx/SingerOS/backend/clientmgr"
)

// Example showing how to send messages to clients from agent code or other services
func AgentSendUpdatesExample() {
	// Wait for client manager to initialize (in a real scenario you'd wait for it to be ready)
	manager := clientmgr.GetDefaultManager()
	if !manager.IsInitialized() {
		// In a real scenario, you'd need to wait for initialization or handle differently
		return
	}

	// Example: Send a status update to a specific client (e.g., client "abc123")
	err := manager.SendAgentStatusUpdate("abc123", "task_456", "processing", "Processing user request...")
	if err != nil {
		// Handle error appropriately
	}

	// Example: Send a step update showing progress
	err = manager.SendAgentStepUpdate("abc123", "task_456", "analyze_code", "Analyzing code changes in PR...")
	if err != nil {
		// Handle error appropriately
	}

	// Example: Send a detailed log message during execution
	err = manager.SendLogMessage("abc123", "task_456", "info", "Successfully parsed code review guidelines")
	if err != nil {
		// Handle error appropriate
	}

	// Example: Send the final result
	err = manager.SendAgentResult("abc123", "task_456", "success", "Code review completed successfully with 2 suggestions")
	if err != nil {
		// Handle error appropriately
	}

	// Example: Broadcast a message to all connected clients (use empty clientID)
	timestamp := time.Now().Unix()
	payload := map[string]interface{}{
		"message":   "System notification: New update available",
		"level":     "info",
		"timestamp": timestamp,
	}

	// Use empty clientID to broadcast to all
	err = manager.SendMessage("", "system_notification", payload)
	if err != nil {
		// Handle error appropriately
	}
}
