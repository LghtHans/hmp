package main

import (
	"fmt"
	"net"
	"time"
)

// SendTimeout is how long the client waits for a response before
// treating a peer as unreachable.
const SendTimeout = 3 * time.Second

// dialPeer opens a UDP "connection" to a peer's last-known address.
// UDP is connectionless, but net.DialUDP still gives us a fixed
// destination + the ability to use Read/Write instead of ReadFrom/WriteTo.
func dialPeer(peer *Peer) (*net.UDPConn, error) {
	if peer.LastKnownIP == "" {
		return nil, fmt.Errorf("hmp: no known address for peer %q — they've never made contact yet", peer.Name)
	}

	addr := net.UDPAddr{
		IP:   net.ParseIP(peer.LastKnownIP),
		Port: peer.LastKnownPort,
	}

	conn, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		return nil, fmt.Errorf("hmp: failed to dial %s: %w", peer.Name, err)
	}
	return conn, nil
}

// SendText sends a live text message to a peer. If the peer is unreachable,
// it queues the message instead of failing silently.
func SendText(peer *Peer, message string) error {
	return sendOrQueue(peer, MsgText, message)
}

// SendMail sends a "mail" message to a peer — semantically the same wire
// behavior as SendText right now (queue on failure), kept as a separate
// function so mail-specific behavior (e.g. different UI treatment) can
// diverge later without touching SendText.
func SendMail(peer *Peer, message string) error {
	return sendOrQueue(peer, MsgText, message)
}

// sendOrQueue attempts to deliver a message directly; on failure it falls
// back to writing it to the local queue for later delivery.
func sendOrQueue(peer *Peer, msgType uint8, content string) error {
	err := sendDirect(peer, msgType, content)
	if err == nil {
		return nil
	}

	// Delivery failed — queue it instead of losing the message.
	qErr := EnqueueMessage(QueuedMessage{
		TargetDeviceID: peer.DeviceID,
		MessageType:    msgType,
		Content:        content,
		QueuedAt:       time.Now().UTC(),
	})
	if qErr != nil {
		return fmt.Errorf("hmp: send failed (%v) and queueing also failed: %w", err, qErr)
	}

	return fmt.Errorf("queued for %s (offline): %w", peer.Name, err)
}

// sendDirect sends one packet straight to a peer's known address, with no
// retry logic. Returns an error if the peer doesn't respond within SendTimeout.
func sendDirect(peer *Peer, msgType uint8, content string) error {
	conn, err := dialPeer(peer)
	if err != nil {
		return err
	}
	defer conn.Close()

	seq := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
	pkt := NewPacket(msgType, seq, []byte(content))
	pkt.SetFlag(FlagRequiresAck)

	data, err := pkt.Encode()
	if err != nil {
		return fmt.Errorf("hmp: failed to encode packet: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("hmp: failed to send to %s: %w", peer.Name, err)
	}

	// Wait for an ACK before declaring success.
	conn.SetReadDeadline(time.Now().Add(SendTimeout))
	buf := make([]byte, ReadBufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("hmp: no response from %s (timed out)", peer.Name)
	}

	ackPkt, err := Decode(buf[:n])
	if err != nil || ackPkt.MessageType != MsgAck {
		return fmt.Errorf("hmp: unexpected response from %s", peer.Name)
	}

	return nil
}

// AnnounceOnline sends an ONLINE packet to every known peer, telling them
// this device is now reachable. The payload carries this device's own
// device_id so the receiving peer knows exactly whose queue to flush.
// Any peer holding queued messages for us should respond by flushing
// their queue toward us.
func AnnounceOnline() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("hmp: failed to load own config: %w", err)
	}

	peers, err := LoadPeers()
	if err != nil {
		return fmt.Errorf("hmp: failed to load peers: %w", err)
	}

	for _, peer := range peers {
		if peer.LastKnownIP == "" {
			continue // never contacted, nothing to announce to
		}

		conn, err := dialPeer(&peer)
		if err != nil {
			continue // skip unreachable peers, don't fail the whole announce
		}

		pkt := NewPacket(MsgOnline, 0, []byte(cfg.DeviceID))
		data, err := pkt.Encode()
		if err != nil {
			conn.Close()
			continue
		}

		conn.Write(data) // best-effort, no ACK required for ONLINE
		conn.Close()
	}

	return nil
}

// FlushQueueTo sends every queued message for a peer, called when that
// peer's ONLINE announcement is received. Clears the queue on full success.
func FlushQueueTo(peer *Peer) error {
	messages, err := LoadQueue(peer.DeviceID)
	if err != nil {
		return fmt.Errorf("hmp: failed to load queue for %s: %w", peer.Name, err)
	}

	if len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		if err := sendDirect(peer, msg.MessageType, msg.Content); err != nil {
			// Stop on first failure — peer may have gone offline again.
			// Remaining messages stay queued for next time.
			return fmt.Errorf("hmp: flush interrupted, %s still offline: %w", peer.Name, err)
		}
	}

	return ClearQueue(peer.DeviceID)
}
