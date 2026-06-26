package protocol

// ControlType represents the type of control action sent to a Worker.
type ControlType string

const (
	// ControlTypeCancelRun requests the Worker to cancel an active agent run.
	ControlTypeCancelRun ControlType = "run.cancel"
)

// WorkerControlBody is the payload of control messages from Server to Worker.
type WorkerControlBody struct {
	ControlType ControlType `json:"control_type"`
	SessionID   string      `json:"session_id"`
	RunID       string      `json:"run_id,omitempty"`
	Reason      string      `json:"reason,omitempty"`
}

// WorkerControlMessage is the control message protocol from Server to Worker.
type WorkerControlMessage = Envelope[WorkerControlBody]
