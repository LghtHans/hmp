package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "-setup":
		cmdSetup()
	case "-add":
		cmdAdd(args[1:])
	case "-listen":
		cmdListen()
	case "-peers":
		cmdPeers()
	case "-status":
		cmdStatus()
	case "-online":
		cmdOnline()
	default:
		// Anything else is assumed to be: hmp <peer_name> -txt|-mail "..."
		cmdMessage(args)
	}
}

func printUsage() {
	fmt.Println(`hmp — usage:

  hmp -setup                        set up this device (name + identity)
  hmp -add <name> <device_id>       add a peer to your address book
  hmp -listen                       start listening in the background
  hmp -peers                        list known peers and their status
  hmp -status                       show this device's identity
  hmp -online                       manually announce that you're online
  hmp <peer_name> -txt              start a live chat session
  hmp <peer_name> -mail "message"   send a message (queued if offline)`)
}

// --- -setup ---

func cmdSetup() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Choose a device name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	if name == "" {
		fmt.Println("  ! name cannot be empty")
		os.Exit(1)
	}

	cfg, err := SetupDevice(name, DefaultPort)
	if err != nil {
		fmt.Printf("  ! setup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ✓ device set up as %q\n", cfg.Name)
	fmt.Printf("  device_id: %s\n", cfg.DeviceID)
	fmt.Printf("  port:      %d\n", cfg.Port)
	fmt.Println("\n  Share your device_id with others so they can add you with 'hmp -add'.")
}

// --- -add ---

func cmdAdd(args []string) {
	if len(args) < 2 {
		fmt.Println("  ! usage: hmp -add <name> <device_id>")
		os.Exit(1)
	}

	name := args[0]
	deviceID := args[1]

	peers, err := LoadPeers()
	if err != nil {
		fmt.Printf("  ! failed to load peers: %v\n", err)
		os.Exit(1)
	}

	if _, err := FindPeerByDeviceID(peers, deviceID); err == nil {
		fmt.Printf("  ! a peer with that device_id already exists\n")
		os.Exit(1)
	}

	peers = append(peers, Peer{
		DeviceID: deviceID,
		Name:     name,
		// LastKnownIP left empty — filled in on first contact.
	})

	if err := SavePeers(peers); err != nil {
		fmt.Printf("  ! failed to save peers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✓ added %q (%s)\n", name, deviceID)
}

// --- -listen ---

func cmdListen() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("  ! %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  starting hmp as %q...\n", cfg.Name)

	// Announce that we're online before settling into the listen loop,
	// so anyone holding queued messages for us flushes them now.
	go func() {
		if err := AnnounceOnline(); err != nil {
			fmt.Printf("  ! online announce failed: %v\n", err)
		}
	}()

	if err := StartServer(cfg.Port); err != nil {
		fmt.Printf("  ! server error: %v\n", err)
		os.Exit(1)
	}
}

// --- -peers ---

func cmdPeers() {
	peers, err := LoadPeers()
	if err != nil {
		fmt.Printf("  ! failed to load peers: %v\n", err)
		os.Exit(1)
	}

	if len(peers) == 0 {
		fmt.Println("  no peers yet — add one with 'hmp -add <name> <device_id>'")
		return
	}

	fmt.Println("\n  NAME      LAST KNOWN ADDRESS       LAST SEEN")
	fmt.Println("  ─────────────────────────────────────────────────")
	for _, p := range peers {
		addr := "—"
		if p.LastKnownIP != "" {
			addr = fmt.Sprintf("%s:%d", p.LastKnownIP, p.LastKnownPort)
		}
		lastSeen := "never"
		if !p.LastSeen.IsZero() {
			lastSeen = p.LastSeen.Format("2006-01-02 15:04")
		}
		fmt.Printf("  %-9s %-24s %s\n", p.Name, addr, lastSeen)
	}
	fmt.Println()
}

// --- -status ---

func cmdStatus() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("  ! %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n  HMP · Device Status")
	fmt.Println("  ────────────────────")
	fmt.Printf("  Name        %s\n", cfg.Name)
	fmt.Printf("  Device ID   %s\n", cfg.DeviceID)
	fmt.Printf("  Port        %d\n\n", cfg.Port)
}

// --- -online ---

func cmdOnline() {
	if err := AnnounceOnline(); err != nil {
		fmt.Printf("  ! %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ announced online to known peers")
}

// --- <peer_name> -txt | -mail ---

func cmdMessage(args []string) {
	if len(args) < 2 {
		printUsage()
		os.Exit(1)
	}

	peerName := args[0]
	medium := args[1]

	peers, err := LoadPeers()
	if err != nil {
		fmt.Printf("  ! failed to load peers: %v\n", err)
		os.Exit(1)
	}

	peer, err := FindPeerByName(peers, peerName)
	if err != nil {
		fmt.Printf("  ! unknown peer %q — add them first with 'hmp -add %s <device_id>'\n", peerName, peerName)
		os.Exit(1)
	}

	switch medium {
	case "-txt":
		runChatSession(peer)
	case "-mail":
		if len(args) < 3 {
			fmt.Println("  ! usage: hmp <peer_name> -mail \"message\"")
			os.Exit(1)
		}
		message := strings.Join(args[2:], " ")
		sendMailOnce(peer, message)
	case "-aud", "-vid":
		fmt.Printf("  ! %s calls are not implemented yet\n", medium)
		os.Exit(1)
	default:
		fmt.Printf("  ! unknown medium %q\n", medium)
		printUsage()
		os.Exit(1)
	}
}

func sendMailOnce(peer *Peer, message string) {
	err := SendMail(peer, message)
	if err != nil {
		// SendMail's error message already says "queued for X (offline)"
		// on the expected failure path, so just print whatever it says.
		fmt.Printf("  %v\n", err)
		return
	}
	fmt.Printf("  ✓ delivered to %s\n", peer.Name)
}

func runChatSession(peer *Peer) {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("  ! %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ─── chat: %s ──────────────────────\n\n", peer.Name)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("  %s$ ", cfg.Name)
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\n")

		if line == "/exit" {
			break
		}
		if line == "" {
			continue
		}

		if err := SendText(peer, line); err != nil {
			fmt.Printf("  ! %v\n", err)
		}
	}
}

// unused for now, kept for when -listen gains a configurable port flag
func parsePort(s string) (int, error) {
	return strconv.Atoi(s)
}
