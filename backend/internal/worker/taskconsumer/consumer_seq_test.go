package taskconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/insmtx/Leros/backend/internal/agent"
	runtimeevents "github.com/insmtx/Leros/backend/internal/runtime/events"
	"github.com/insmtx/Leros/backend/internal/worker/protocol"
	"github.com/insmtx/Leros/backend/pkg/leros"
	"github.com/nats-io/nats.go"
)

type fakeSeqTracker struct {
	lastTerminal uint64
	terminal     map[uint64]bool
	received     []uint64
	processing   []uint64
	completed    []uint64
	failed       map[uint64]string
}

func (f *fakeSeqTracker) TrackReceived(_ context.Context, _ string, seq uint64, _, _, _, _ string) error {
	f.received = append(f.received, seq)
	return nil
}

func (f *fakeSeqTracker) MarkProcessing(_ context.Context, _ string, seq uint64) error {
	f.processing = append(f.processing, seq)
	return nil
}

func (f *fakeSeqTracker) MarkCompleted(_ context.Context, _ string, seq uint64) error {
	f.completed = append(f.completed, seq)
	return nil
}

func (f *fakeSeqTracker) MarkFailed(_ context.Context, _ string, seq uint64, errMsg string) error {
	if f.failed == nil {
		f.failed = make(map[uint64]string)
	}
	f.failed[seq] = errMsg
	return nil
}

func (f *fakeSeqTracker) GetLastCompletedSeq(context.Context, string) (uint64, error) {
	return 0, nil
}

func (f *fakeSeqTracker) GetLastTerminalSeq(context.Context, string) (uint64, error) {
	return f.lastTerminal, nil
}

func (f *fakeSeqTracker) IsDuplicate(context.Context, string, uint64) (bool, error) {
	return false, nil
}

func (f *fakeSeqTracker) IsTerminal(_ context.Context, _ string, seq uint64) (bool, error) {
	return f.terminal[seq], nil
}

func (f *fakeSeqTracker) Close() error {
	return nil
}

type fakeSubscriber struct {
	subscribeCalled     bool
	subscribeFromCalled bool
	startSeq            int64
}

func (f *fakeSubscriber) Subscribe(context.Context, string, string, func(*nats.Msg)) error {
	f.subscribeCalled = true
	return nil
}

func (f *fakeSubscriber) SubscribeFrom(_ context.Context, _ string, startSeq int64, _ func(*nats.Msg)) error {
	f.subscribeFromCalled = true
	f.startSeq = startSeq
	return nil
}

type fakePublisher struct {
	calls []publishedEvent
}

type publishedEvent struct {
	topic string
	event any
}

func (f *fakePublisher) Publish(_ context.Context, topic string, event any) error {
	f.calls = append(f.calls, publishedEvent{topic: topic, event: event})
	return nil
}

func (f *fakePublisher) Request(context.Context, string, any) (*nats.Msg, error) {
	return nil, nil
}

type fakeRunner struct {
	err    error
	result *agent.RunResult
	emit   *runtimeevents.Event
	calls  int
}

func (f *fakeRunner) Run(ctx context.Context, req *agent.RequestContext) (*agent.RunResult, error) {
	f.calls++
	if f.emit != nil && req.EventSink != nil {
		_ = req.EventSink.Emit(ctx, f.emit)
	}
	if f.err != nil {
		return f.result, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &agent.RunResult{RunID: req.RunID, Status: agent.RunStatusCompleted}, nil
}

func TestConsumerStartRecoversFromLastTerminalSeq(t *testing.T) {
	subscriber := &fakeSubscriber{}
	consumer := &Consumer{
		cfg:        Config{OrgID: 1, WorkerID: 2},
		subscriber: subscriber,
		seqTracker: &fakeSeqTracker{lastTerminal: 42},
	}

	if err := consumer.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !subscriber.subscribeFromCalled {
		t.Fatal("expected SubscribeFrom to be used for terminal seq recovery")
	}
	if subscriber.startSeq != 43 {
		t.Fatalf("start seq = %d, want 43", subscriber.startSeq)
	}
	if subscriber.subscribeCalled {
		t.Fatal("did not expect normal Subscribe when terminal seq exists")
	}
}

func TestConsumerHandleEventSkipsTerminalSeq(t *testing.T) {
	tracker := &fakeSeqTracker{terminal: map[uint64]bool{0: true}}
	runner := &fakeRunner{}
	consumer := &Consumer{
		cfg:        Config{OrgID: 1, WorkerID: 2},
		publisher:  &fakePublisher{},
		runner:     runner,
		seqTracker: tracker,
		sem:        make(chan struct{}, 1),
	}

	data, err := json.Marshal(testWorkerTaskMessage())
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	if err := consumer.handleEvent(context.Background(), &nats.Msg{Data: data}); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}

	if len(tracker.received) != 0 {
		t.Fatalf("terminal message should not be tracked as received, got %v", tracker.received)
	}
	if runner.calls != 0 {
		t.Fatalf("terminal message should not run, calls=%d", runner.calls)
	}
	if len(consumer.sem) != 0 {
		t.Fatal("semaphore slot was not released")
	}
}

func TestConsumerExecuteWithTrackerMarksAllSeqsFailed(t *testing.T) {
	t.Setenv(leros.EnvWorkspaceRoot, t.TempDir())

	runErr := errors.New("skill not found")
	tracker := &fakeSeqTracker{}
	publisher := &fakePublisher{}
	consumer := &Consumer{
		cfg:        Config{OrgID: 1, WorkerID: 2},
		publisher:  publisher,
		runner:     &fakeRunner{err: runErr},
		seqTracker: tracker,
	}
	msg := testWorkerTaskMessage()
	msg.Route.SessionID = "session_1"
	setSeqs(&msg, []uint64{7, 8})

	err := consumer.executeWithTracker(context.Background(), msg)
	if !errors.Is(err, runErr) {
		t.Fatalf("executeWithTracker error = %v, want %v", err, runErr)
	}

	if !sameSeqs(tracker.processing, []uint64{7, 8}) {
		t.Fatalf("processing seqs = %v, want [7 8]", tracker.processing)
	}
	for _, seq := range []uint64{7, 8} {
		if tracker.failed[seq] != runErr.Error() {
			t.Fatalf("failed[%d] = %q, want %q", seq, tracker.failed[seq], runErr.Error())
		}
	}
	if len(tracker.completed) != 0 {
		t.Fatalf("completed seqs = %v, want none", tracker.completed)
	}
	if len(publisher.calls) != 2 {
		t.Fatalf("published events = %d, want stream and completed failure events", len(publisher.calls))
	}
	if streamMsg, ok := publisher.calls[0].event.(protocol.MessageStreamMessage); !ok ||
		streamMsg.Body.Event != protocol.StreamEventRunFailed {
		t.Fatalf("first published event = %#v, want run.failed stream event", publisher.calls[0].event)
	}
}

func TestConsumerExecuteWithTrackerDoesNotEmitRunFailedForCancelledRun(t *testing.T) {
	t.Setenv(leros.EnvWorkspaceRoot, t.TempDir())

	tracker := &fakeSeqTracker{}
	publisher := &fakePublisher{}
	cancelledEvent := runtimeevents.NewRunCompleted(runtimeevents.RunCompletedPayload{
		Status: string(agent.RunStatusCancelled),
		Result: runtimeevents.RunResultPayload{
			Message: "已取消",
		},
	}, "已取消")
	cancelledEvent.Type = runtimeevents.EventCancelled
	consumer := &Consumer{
		cfg:       Config{OrgID: 1, WorkerID: 2},
		publisher: publisher,
		runner: &fakeRunner{
			err: context.Canceled,
			result: &agent.RunResult{
				RunID:  "run_1",
				Status: agent.RunStatusCancelled,
				Error:  context.Canceled.Error(),
			},
			emit: cancelledEvent,
		},
		seqTracker: tracker,
	}
	msg := testWorkerTaskMessage()
	msg.Route.SessionID = "session_1"
	setSeqs(&msg, []uint64{7})

	if err := consumer.executeWithTracker(context.Background(), msg); err != nil {
		t.Fatalf("executeWithTracker error = %v, want nil for cancellation", err)
	}

	if !sameSeqs(tracker.completed, []uint64{7}) {
		t.Fatalf("completed seqs = %v, want [7]", tracker.completed)
	}
	if len(tracker.failed) != 0 {
		t.Fatalf("failed seqs = %#v, want none", tracker.failed)
	}
	if len(publisher.calls) != 2 {
		t.Fatalf("published events = %d, want stream and completed cancellation events", len(publisher.calls))
	}
	for _, call := range publisher.calls {
		streamMsg, ok := call.event.(protocol.MessageStreamMessage)
		if !ok {
			t.Fatalf("published event = %#v, want MessageStreamMessage", call.event)
		}
		if streamMsg.Body.Event == protocol.StreamEventRunFailed {
			t.Fatalf("published run.failed for cancellation: %#v", streamMsg)
		}
	}
}

func TestConsumerExecuteWithTrackerEmitsRunFailedWhenPrepareWorkspaceFails(t *testing.T) {
	t.Setenv(leros.EnvWorkspaceRoot, t.TempDir())

	tracker := &fakeSeqTracker{}
	publisher := &fakePublisher{}
	consumer := &Consumer{
		cfg:        Config{OrgID: 1, WorkerID: 2},
		publisher:  publisher,
		runner:     &fakeRunner{},
		seqTracker: tracker,
	}
	msg := testWorkerTaskMessage()
	msg.Route.SessionID = "session_1"
	msg.Body.Workspace.ProjectID = "project_1"
	// 使用越界 work_dir 触发 workspace 准备失败，验证失败事件仍会补发。
	msg.Body.Runtime.WorkDir = "../escape"
	setSeqs(&msg, []uint64{9})

	err := consumer.executeWithTracker(context.Background(), msg)
	if err == nil {
		t.Fatal("executeWithTracker error = nil, want workspace prepare error")
	}

	if tracker.failed[9] == "" {
		t.Fatalf("failed seq 9 should be recorded, got %q", tracker.failed[9])
	}
	if len(publisher.calls) != 2 {
		t.Fatalf("published events = %d, want stream and completed failure events", len(publisher.calls))
	}
	if streamMsg, ok := publisher.calls[0].event.(protocol.MessageStreamMessage); !ok ||
		streamMsg.Body.Event != protocol.StreamEventRunFailed {
		t.Fatalf("first published event = %#v, want run.failed stream event", publisher.calls[0].event)
	}
}

func testWorkerTaskMessage() protocol.WorkerTaskMessage {
	return protocol.WorkerTaskMessage{
		ID:        "msg_1",
		Type:      protocol.MessageTypeWorkerTask,
		CreatedAt: time.Now().UTC(),
		Trace: protocol.TraceContext{
			TraceID: "trace_1",
			TaskID:  "task_1",
			RunID:   "run_1",
		},
		Route: protocol.RouteContext{
			OrgID:    1,
			WorkerID: 2,
		},
		Body: protocol.WorkerTaskBody{
			TaskType: protocol.TaskTypeAgentRun,
			Input: protocol.TaskInput{
				Type: protocol.InputTypeMessage,
				Messages: []protocol.ChatMessage{
					{Role: protocol.MessageRoleUser, Content: "hello"},
				},
			},
			Model: protocol.ModelOptions{
				Provider: "openai",
				Model:    "gpt-4.1",
				APIKey:   "test-key",
			},
		},
	}
}

func sameSeqs(got []uint64, want []uint64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
