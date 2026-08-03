package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Peer represents one known device in the fixed HMP group.
type Peer struct {
	DeviceID       string    `json:"device_id"`
	Name           string    `json:"name"`
	LastKnownIP    string    `json:"last_known_ip"`
	LastKnownPort  int       `json:"last_known_port"`
	LastSeen       time.Time `json:"last_seen"`
}

// peerFile is the on-disk shape of peers.json.
type peerFile struct {
	Peers []Peer `json:"peers"`
}

// ErrPeerNotFound is returned when a lookup by name or device ID fails.
var ErrPeerNotFound = errors.New("hmp: peer not found")

// hmpDir returns the path to ~/.hmp, creating it if it doesn't exist.
func hmpDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".hmp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// peersPath returns the full path to peers.json.
func peersPath() (string, error) {
	dir, err := hmpDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "peers.json"), nil
}

// LoadPeers reads peers.json from disk. If the file doesn't exist yet,
// it returns an empty peer list (not an error) so a fresh install works.
func LoadPeers() ([]Peer, error) {
	path, err := peersPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Peer{}, nil
		}
		return nil, err
	}

	var pf peerFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, err
	}
	return pf.Peers, nil
}

// SavePeers writes the full peer list back to peers.json, overwriting it.
func SavePeers(peers []Peer) error {
	path, err := peersPath()
	if err != nil {
		return err
	}

	pf := peerFile{Peers: peers}
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// FindPeerByName looks up a peer by its display name (case-sensitive).
func FindPeerByName(peers []Peer, name string) (*Peer, error) {
	for i := range peers {
		if peers[i].Name == name {
			return &peers[i], nil
		}
	}
	return nil, ErrPeerNotFound
}

// FindPeerByDeviceID looks up a peer by its UUID.
func FindPeerByDeviceID(peers []Peer, deviceID string) (*Peer, error) {
	for i := range peers {
		if peers[i].DeviceID == deviceID {
			return &peers[i], nil
		}
	}
	return nil, ErrPeerNotFound
}

// UpdatePeerAddress updates (or inserts) a peer's last-known IP/port/last-seen.
// This is what keeps the address book "self-healing" — call it whenever a
// packet is successfully received from or sent to a peer.
func UpdatePeerAddress(peers []Peer, deviceID, name, ip string, port int) []Peer {
	for i := range peers {
		if peers[i].DeviceID == deviceID {
			peers[i].LastKnownIP = ip
			peers[i].LastKnownPort = port
			peers[i].LastSeen = time.Now().UTC()
			return peers
		}
	}

	// Not found — this is a new peer, add it.
	peers = append(peers, Peer{
		DeviceID:      deviceID,
		Name:          name,
		LastKnownIP:   ip,
		LastKnownPort: port,
		LastSeen:      time.Now().UTC(),
	})
	return peers
}

