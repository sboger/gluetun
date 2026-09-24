package bittorrent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/qdm12/gluetun/internal/configuration/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mutexPortForwarder struct {
	mu    sync.Mutex
	ports []uint16
}

func (m *mutexPortForwarder) GetPortsForwarded() (ports []uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ports
}

func (m *mutexPortForwarder) setPorts(ports []uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ports = ports
}

type discardingLogger struct{}

func (discardingLogger) Info(string)  {}
func (discardingLogger) Error(string) {}

func Test_Loop_resolvePort_manual(t *testing.T) {
	t.Parallel()

	const port = uint16(51413)
	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
			Port:    ptrTo(port),
		},
	}, &mutexPortForwarder{}, discardingLogger{})

	got, err := loop.resolvePort(context.Background())

	require.NoError(t, err)
	assert.Equal(t, port, got)
}

func Test_Loop_resolvePort_random_when_no_forwarding(t *testing.T) {
	t.Parallel()

	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
		},
		UseForwardedPort: false,
	}, &mutexPortForwarder{}, discardingLogger{})

	got, err := loop.resolvePort(context.Background())

	require.NoError(t, err)
	assert.Equal(t, uint16(0), got)
}

func Test_Loop_resolvePort_forwarded_available(t *testing.T) {
	t.Parallel()

	portForwarder := &mutexPortForwarder{}
	portForwarder.setPorts([]uint16{6881})
	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
		},
		UseForwardedPort: true,
	}, portForwarder, discardingLogger{})

	got, err := loop.resolvePort(context.Background())

	require.NoError(t, err)
	assert.Equal(t, uint16(6881), got)
}

func Test_Loop_resolvePort_forwarded_canceled(t *testing.T) {
	t.Parallel()

	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
		},
		UseForwardedPort: true,
	}, &mutexPortForwarder{}, discardingLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := loop.resolvePort(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Loop_Run_disabled(t *testing.T) {
	t.Parallel()

	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
		},
	}, &mutexPortForwarder{}, discardingLogger{})

	done := make(chan struct{})
	go loop.Run(context.Background(), done)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return for a disabled BitTorrent client")
	}
}

func Test_Loop_AddRemove_when_not_running(t *testing.T) {
	t.Parallel()

	loop := NewLoop(Settings{
		Bittorrent: settings.Bittorrent{
			Enabled: ptrTo(false),
		},
	}, &mutexPortForwarder{}, discardingLogger{})

	_, err := loop.AddTorrent("magnet:?xt=urn:btih:deadbeef")
	assert.ErrorContains(t, err, "not running")

	assert.Nil(t, loop.ListTorrents())

	err = loop.RemoveTorrent("deadbeef")
	assert.ErrorContains(t, err, "not running")
}

func ptrTo[T any](value T) *T {
	return &value
}
