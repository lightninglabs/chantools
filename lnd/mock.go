package lnd

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/peer"
	"github.com/stretchr/testify/require"
)

const (
	timeout = time.Second * 5
)

// mockMessageSwitch is a mock implementation of the messageSwitch interface
// used for testing without relying on a *htlcswitch.Switch in unit tests.
type mockMessageSwitch struct {
	links []htlcswitch.ChannelUpdateHandler
}

// BestHeight currently returns a dummy value.
func (m *mockMessageSwitch) BestHeight() uint32 {
	return 0
}

// CircuitModifier currently returns a dummy value.
func (m *mockMessageSwitch) CircuitModifier() htlcswitch.CircuitModifier {
	return nil
}

// RemoveLink currently does nothing.
func (m *mockMessageSwitch) RemoveLink(cid lnwire.ChannelID) {}

// CreateAndAddLink currently returns a dummy value.
func (m *mockMessageSwitch) CreateAndAddLink(cfg htlcswitch.ChannelLinkConfig,
	lnChan *lnwallet.LightningChannel) error {

	return nil
}

// GetLinksByInterface returns the active links.
func (m *mockMessageSwitch) GetLinksByInterface(pub [33]byte) (
	[]htlcswitch.ChannelUpdateHandler, error) {

	return m.links, nil
}

// mockUpdateHandler is a mock implementation of the ChannelUpdateHandler
// interface. It is used in mockMessageSwitch's GetLinksByInterface method.
type mockUpdateHandler struct {
	cid                  lnwire.ChannelID
	isOutgoingAddBlocked atomic.Bool
	isIncomingAddBlocked atomic.Bool
}

// newMockUpdateHandler creates a new mockUpdateHandler.
func newMockUpdateHandler(cid lnwire.ChannelID) *mockUpdateHandler {
	return &mockUpdateHandler{
		cid: cid,
	}
}

// HandleChannelUpdate currently does nothing.
func (m *mockUpdateHandler) HandleChannelUpdate(msg lnwire.Message) {}

// ChanID returns the mockUpdateHandler's cid.
func (m *mockUpdateHandler) ChanID() lnwire.ChannelID { return m.cid }

// Bandwidth currently returns a dummy value.
func (m *mockUpdateHandler) Bandwidth() lnwire.MilliSatoshi { return 0 }

// EligibleToForward currently returns a dummy value.
func (m *mockUpdateHandler) EligibleToForward() bool { return false }

// MayAddOutgoingHtlc currently returns nil.
func (m *mockUpdateHandler) MayAddOutgoingHtlc(lnwire.MilliSatoshi) error { return nil }

type mockMessageConn struct {
	t *testing.T

	// MessageConn embeds our interface so that the mock does not need to
	// implement every function. The mock will panic if an unspecified function
	// is called.
	peer.MessageConn

	// writtenMessages is a channel that our mock pushes written messages into.
	writtenMessages chan []byte

	readMessages   chan []byte
	curReadMessage []byte

	// writeRaceDetectingCounter is incremented on any function call
	// associated with writing to the connection. The race detector will
	// trigger on this counter if a data race exists.
	writeRaceDetectingCounter int

	// readRaceDetectingCounter is incremented on any function call
	// associated with reading from the connection. The race detector will
	// trigger on this counter if a data race exists.
	readRaceDetectingCounter int
}

func (m *mockUpdateHandler) EnableAdds(dir htlcswitch.LinkDirection) bool {
	if dir == htlcswitch.Outgoing {
		return m.isOutgoingAddBlocked.Swap(false)
	}

	return m.isIncomingAddBlocked.Swap(false)
}

func (m *mockUpdateHandler) DisableAdds(dir htlcswitch.LinkDirection) bool {
	if dir == htlcswitch.Outgoing {
		return !m.isOutgoingAddBlocked.Swap(true)
	}

	return !m.isIncomingAddBlocked.Swap(true)
}

func (m *mockUpdateHandler) IsFlushing(dir htlcswitch.LinkDirection) bool {
	switch dir {
	case htlcswitch.Outgoing:
		return m.isOutgoingAddBlocked.Load()
	case htlcswitch.Incoming:
		return m.isIncomingAddBlocked.Load()
	}

	return false
}

func (m *mockUpdateHandler) OnFlushedOnce(hook func()) {
	hook()
}
func (m *mockUpdateHandler) OnCommitOnce(
	_ htlcswitch.LinkDirection, hook func(),
) {

	hook()
}
func (m *mockUpdateHandler) InitStfu() <-chan fn.Result[lntypes.ChannelParty] {
	// TODO(proofofkeags): Implement
	c := make(chan fn.Result[lntypes.ChannelParty], 1)

	c <- fn.Errf[lntypes.ChannelParty]("InitStfu not yet implemented")

	return c
}

func newMockConn(t *testing.T, expectedMessages int) *mockMessageConn {
	return &mockMessageConn{
		t:               t,
		writtenMessages: make(chan []byte, expectedMessages),
		readMessages:    make(chan []byte, 1),
	}
}

// SetWriteDeadline mocks setting write deadline for our conn.
func (m *mockMessageConn) SetWriteDeadline(time.Time) error {
	m.writeRaceDetectingCounter++
	return nil
}

// Flush mocks a message conn flush.
func (m *mockMessageConn) Flush() (int, error) {
	m.writeRaceDetectingCounter++
	return 0, nil
}

// WriteMessage mocks sending of a message on our connection. It will push
// the bytes sent into the mock's writtenMessages channel.
func (m *mockMessageConn) WriteMessage(msg []byte) error {
	m.writeRaceDetectingCounter++

	msgCopy := make([]byte, len(msg))
	copy(msgCopy, msg)

	select {
	case m.writtenMessages <- msgCopy:
	case <-time.After(timeout):
		m.t.Fatalf("timeout sending message: %v", msgCopy)
	}

	return nil
}

// assertWrite asserts that our mock as had WriteMessage called with the byte
// slice we expect.
func (m *mockMessageConn) assertWrite(expected []byte) {
	select {
	case actual := <-m.writtenMessages:
		require.Equal(m.t, expected, actual)

	case <-time.After(timeout):
		m.t.Fatalf("timeout waiting for write: %v", expected)
	}
}

func (m *mockMessageConn) SetReadDeadline(t time.Time) error {
	m.readRaceDetectingCounter++
	return nil
}

func (m *mockMessageConn) ReadNextHeader() (uint32, error) {
	m.readRaceDetectingCounter++
	m.curReadMessage = <-m.readMessages
	return uint32(len(m.curReadMessage)), nil
}

func (m *mockMessageConn) ReadNextBody(buf []byte) ([]byte, error) {
	m.readRaceDetectingCounter++
	return m.curReadMessage, nil
}

func (m *mockMessageConn) RemoteAddr() net.Addr {
	return nil
}

func (m *mockMessageConn) LocalAddr() net.Addr {
	return nil
}

func (m *mockMessageConn) Close() error {
	return nil
}
