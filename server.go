package main

import (
	"fmt"
	"net"
)

// DefaultPort is the UDP port HMP listens on unless overridden.
const DefaultPort = 51820

// ReadBufferSize is the max size of a single incoming UDP datagram we accept.
const ReadBufferSize = 65535

// StartServer opens a UDP socket on the given port and processes incoming
// HMP packets forever. It never returns unless the socket fails to open
// or a fatal read error occurs.
func StartServer(port int) error {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.IPv4zero, // listen on all interfaces
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return fmt.Errorf("hmp: failed to start server on port %d: %w", port, err)
	}
	defer conn.Close()

	fmt.Printf("  ● listening on port %d\n", port)

	buf := make([]byte, ReadBufferSize)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// A single bad read shouldn't kill the whole server.
			fmt.Printf("  ! read error from %v: %v\n", remoteAddr, err)
			continue
		}

		// Copy out of buf since it gets reused on the next iteration.
		data := make([]byte, n)
		copy(data, buf[:n])

		go handlePacket(data, remoteAddr, conn)
	}
}

// handlePacket decodes a raw UDP datagram and dispatches it based on
// message type. Runs in its own goroutine so a slow handler for one
// packet never blocks the read loop for the next.
func handlePacket(data []byte, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	pkt, err := Decode(data)
	if err != nil {
		fmt.Printf("  ! dropped invalid packet from %v: %v\n", remoteAddr, err)
		return
	}

	switch pkt.MessageType {
	case MsgHello:
		handleHello(pkt, remoteAddr, conn)
	case MsgHelloAck:
		handleHelloAck(pkt, remoteAddr)
	case MsgText:
		handleText(pkt, remoteAddr)
	case MsgAck:
		handleAck(pkt, remoteAddr)
	case MsgOnline:
		handleOnline(pkt, remoteAddr)
	case MsgPing:
		handlePing(pkt, remoteAddr, conn)
	case MsgPong:
		// keepalive response, nothing to do yet
	case MsgBye:
		handleBye(pkt, remoteAddr)
	case MsgFileStart, MsgFileChunk, MsgFileEnd:
		// File transfer handling comes later — not yet implemented.
		fmt.Printf("  ! file transfer packets not yet supported (from %v)\n", remoteAddr)
	default:
		fmt.Printf("  ! unknown message type 0x%02X from %v\n", pkt.MessageType, remoteAddr)
	}
}

// --- Individual message type handlers (stubs to be filled in as we build peers/client) ---

func handleHello(pkt *Packet, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	// TODO: parse device_id/device_name from payload, update peers.json,
	// then send back a HELLO_ACK.
	fmt.Printf("  → HELLO received from %v\n", remoteAddr)
}

func handleHelloAck(pkt *Packet, remoteAddr *net.UDPAddr) {
	fmt.Printf("  → HELLO_ACK received from %v\n", remoteAddr)
}

func handleText(pkt *Packet, remoteAddr *net.UDPAddr) {
	// TODO: once peers.json exists, resolve remoteAddr -> peer name
	// instead of printing the raw address.
	fmt.Printf("  %v$ %s\n", remoteAddr, string(pkt.Payload))
}

func handleAck(pkt *Packet, remoteAddr *net.UDPAddr) {
	// TODO: mark pkt.Sequence as acknowledged in the client's pending-ack tracker.
	fmt.Printf("  → ACK for seq %d from %v\n", pkt.Sequence, remoteAddr)
}

func handleOnline(pkt *Packet, remoteAddr *net.UDPAddr) {
	deviceID := string(pkt.Payload)
	if deviceID == "" {
		fmt.Printf("  ! ONLINE packet from %v missing device_id, ignoring\n", remoteAddr)
		return
	}

	peers, err := LoadPeers()
	if err != nil {
		fmt.Printf("  ! failed to load peers: %v\n", err)
		return
	}

	peer, err := FindPeerByDeviceID(peers, deviceID)
	if err != nil {
		// Not someone we know — ignore. In a fixed 20-person group this
		// shouldn't normally happen unless peers.json is out of sync.
		fmt.Printf("  ! ONLINE from unknown device_id %s (%v)\n", deviceID, remoteAddr)
		return
	}

	// Update their address in case it changed since we last heard from them.
	peers = UpdatePeerAddress(peers, peer.DeviceID, peer.Name, remoteAddr.IP.String(), remoteAddr.Port)
	if err := SavePeers(peers); err != nil {
		fmt.Printf("  ! failed to save updated peer address: %v\n", err)
	}

	fmt.Printf("  → %s is now online\n", peer.Name)

	// Re-fetch the updated peer pointer before flushing, so we send to
	// the freshest known address.
	updatedPeers, _ := LoadPeers()
	updatedPeer, err := FindPeerByDeviceID(updatedPeers, deviceID)
	if err != nil {
		return
	}

	if err := FlushQueueTo(updatedPeer); err != nil {
		fmt.Printf("  ! queue flush to %s failed: %v\n", updatedPeer.Name, err)
	}
}

func handlePing(pkt *Packet, remoteAddr *net.UDPAddr, conn *net.UDPConn) {
	pong := NewPacket(MsgPong, pkt.Sequence, pkt.Payload)
	data, err := pong.Encode()
	if err != nil {
		fmt.Printf("  ! failed to encode PONG: %v\n", err)
		return
	}
	if _, err := conn.WriteToUDP(data, remoteAddr); err != nil {
		fmt.Printf("  ! failed to send PONG to %v: %v\n", remoteAddr, err)
	}
}

func handleBye(pkt *Packet, remoteAddr *net.UDPAddr) {
	fmt.Printf("  → %v disconnected\n", remoteAddr)
}
