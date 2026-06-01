package core

import (
	"sync"
	"testing"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

func noopFactory(cfg types.ChannelConfig) (any, error) {
	return &dummyAdapter{info: types.AdapterInfo{Code: cfg.Code}}, nil
}

type dummyAdapter struct {
	info    types.AdapterInfo
	cb      MessageCallback
	connect bool
}

func (d *dummyAdapter) Info() types.AdapterInfo                         { return d.info }
func (d *dummyAdapter) Connect(ctx struct{}) error                      { d.connect = true; return nil }
func (d *dummyAdapter) Disconnect(ctx struct{}) error                   { d.connect = false; return nil }
func (d *dummyAdapter) OnMessage(cb MessageCallback) error              { d.cb = cb; return nil }
func (d *dummyAdapter) Send(ctx struct{}, t string, m types.OutboundMessage) error { return nil }
func (d *dummyAdapter) SendTyping(ctx struct{}, t string) error         { return nil }

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	err := r.Register(ChannelEntry{Code: "feishu", Factory: noopFactory})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if r.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", r.Len())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(ChannelEntry{Code: "feishu", Factory: noopFactory})

	err := r.Register(ChannelEntry{Code: "feishu", Factory: noopFactory})
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_RegisterEmptyCode(t *testing.T) {
	r := NewRegistry()
	err := r.Register(ChannelEntry{Code: "", Factory: noopFactory})
	if err == nil {
		t.Error("expected error for empty code")
	}
}

func TestRegistry_RegisterNilFactory(t *testing.T) {
	r := NewRegistry()
	err := r.Register(ChannelEntry{Code: "feishu"})
	if err == nil {
		t.Error("expected error for nil factory")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	entry := ChannelEntry{Code: "feishu", Label: "Feishu", Factory: noopFactory}
	r.MustRegister(entry)

	got := r.Get("feishu")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Label != "Feishu" {
		t.Errorf("expected label Feishu, got %s", got.Label)
	}

	if r.Get("unknown") != nil {
		t.Error("Get(unknown) should return nil")
	}
}

func TestRegistry_List_Sorted(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(ChannelEntry{Code: "github", Order: 200, Factory: noopFactory})
	r.MustRegister(ChannelEntry{Code: "feishu", Order: 100, Factory: noopFactory})
	r.MustRegister(ChannelEntry{Code: "wecom", Order: 150, Factory: noopFactory})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}

	// Ordered by Order asc, then by Code asc
	if list[0].Code != "feishu" {
		t.Errorf("expected feishu first, got %s", list[0].Code)
	}
	if list[1].Code != "wecom" {
		t.Errorf("expected wecom second, got %s", list[1].Code)
	}
	if list[2].Code != "github" {
		t.Errorf("expected github third, got %s", list[2].Code)
	}
}

func TestRegistry_List_SameOrderAlphabetical(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(ChannelEntry{Code: "github", Order: 100, Factory: noopFactory})
	r.MustRegister(ChannelEntry{Code: "feishu", Order: 100, Factory: noopFactory})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Code != "feishu" || list[1].Code != "github" {
		t.Errorf("same-order entries should sort alphabetically: %v", list)
	}
}

func TestRegistry_Enabled(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(ChannelEntry{
		Code:    "feishu",
		Enabled: func() bool { return true },
		Factory: noopFactory,
	})
	r.MustRegister(ChannelEntry{
		Code:    "github",
		Enabled: func() bool { return false },
		Factory: noopFactory,
	})
	r.MustRegister(ChannelEntry{
		Code:    "wecom",
		Enabled: nil, // nil == always enabled
		Factory: noopFactory,
	})

	enabled := r.Enabled()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled entries, got %d", len(enabled))
	}

	codes := make(map[types.ChannelCode]bool)
	for _, e := range enabled {
		codes[e.Code] = true
	}
	if !codes["feishu"] {
		t.Error("feishu should be enabled")
	}
	if codes["github"] {
		t.Error("github should be disabled")
	}
	if !codes["wecom"] {
		t.Error("wecom should be enabled (nil Enabled)")
	}
}

func TestRegistry_Codes(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(ChannelEntry{Code: "github", Factory: noopFactory})
	r.MustRegister(ChannelEntry{Code: "feishu", Factory: noopFactory})

	codes := r.Codes()
	if len(codes) != 2 {
		t.Fatalf("expected 2 codes, got %d", len(codes))
	}
	if codes[0] != "feishu" || codes[1] != "github" {
		t.Errorf("unexpected order: %v", codes)
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	channels := []types.ChannelCode{"a", "b", "c", "d", "e"}
	for _, code := range channels {
		wg.Add(1)
		go func(c types.ChannelCode) {
			defer wg.Done()
			_ = r.Register(ChannelEntry{Code: c, Factory: noopFactory})
		}(code)
	}
	wg.Wait()

	if r.Len() != len(channels) {
		t.Errorf("expected %d entries, got %d", len(channels), r.Len())
	}
}

func TestRegistry_MustRegisterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustRegister with empty code should panic")
		}
	}()
	r := NewRegistry()
	r.MustRegister(ChannelEntry{Code: "", Factory: noopFactory})
}
