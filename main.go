package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
)

// Protocol Message Structure
type Message struct {
	Type      string `json:"type"` // "CHAT", "FILE_OFFER", "FILE_ACCEPT", "FILE_REJECT", "FILE_DATA"
	Content   string `json:"content,omitempty"`
	FileName  string `json:"fileName,omitempty"`
	FileSize  int64  `json:"fileSize,omitempty"`
	FileBytes []byte `json:"fileBytes,omitempty"`
	Sender    string `json:"sender,omitempty"` // optional display name
}

// App State
var defaultDownloadDir = "."
var displayName, peerName string
var pendingOffer *Message
var waitingForAcceptance = false
var localDataChannel *webrtc.DataChannel
var senderFilePath string // Tracks file waiting to be sent upon acceptance

// Input History
var history []string
var histCursor int
var currentLine bytes.Buffer

const (
	historyMax = 50
	upArrow    = "\x1b[A"
	downArrow  = "\x1b[B"
)

func main() {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	mode := os.Args[1]

	switch mode {
	case "-h", "--help":
		printHelp()
		return
	case "-v", "--version":
		printVersion()
		return
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer peerConnection.Close()

	if mode == "listen" || mode == "host" {
		// 1. Set the default fallback value first
		listenerPort := "8080"

		// 2. Safely check if the argument was actually provided
		if len(os.Args) >= 3 {
			listenerPort = os.Args[2]
		} else {
			// Optional: Warn the user but gracefully fall back to the default
			fmt.Println("No port provided. Defaulting to 8080. Usage: lan-p2p listen 8090")
		}

		listener, err := net.Listen("tcp", ":"+listenerPort)
		if err != nil {
			panic(err)
		}
		defer listener.Close()

		fmt.Printf("[*] Listening for connection on LAN port %s...\n", listenerPort)
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		fmt.Println("[*] Peer connected! Exchanging WebRTC metadata...")

		peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
			localDataChannel = dc
			setupDataChannel(dc)
		})

		reader := bufio.NewReader(conn)
		offerStr, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}

		offer := &webrtc.SessionDescription{}
		DecodeFromString(offerStr, offer)

		if err := peerConnection.SetRemoteDescription(*offer); err != nil {
			panic(err)
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		if err := peerConnection.SetLocalDescription(answer); err != nil {
			panic(err)
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
		<-gatherComplete

		conn.Write([]byte(EncodeToString(peerConnection.LocalDescription()) + "\n"))
		fmt.Println("[*] Handshake complete! Waiting for DataChannel to open...")

	} else if mode == "connect" || mode == "join" {
		// 1. Set the default fallback values first
		receiverIP := "127.0.0.1"
		receiverPort := "8080"

		// 2. Safely check if the argument was actually provided
		if len(os.Args) >= 3 {
			input := os.Args[2]

			// net.SplitHostPort expects a colon. If missing (e.g., "192.168.1.12"),
			// it treats the entire string as the host/IP.
			if !strings.Contains(input, ":") {
				receiverIP = input
			} else {
				// If a colon is present, split it into IP and Port
				host, port, err := net.SplitHostPort(input)
				if err == nil {
					// Only override defaults if the extracted parts are not empty
					if host != "" {
						receiverIP = host
					}
					if port != "" {
						receiverPort = port
					}
				} else {
					fmt.Printf("Warning: Invalid format '%s'. Using defaults.\n", input)
				}
			}
		} else {
			fmt.Println("No address provided. Defaulting to 127.0.0.1:8080. Usage: lan-p2p connect 192.168.1.12:8090")
		}

		dataChannel, err := peerConnection.CreateDataChannel("p2p-data", nil)
		if err != nil {
			panic(err)
		}
		localDataChannel = dataChannel
		setupDataChannel(dataChannel)

		offer, err := peerConnection.CreateOffer(nil)
		if err != nil {
			panic(err)
		}
		if err := peerConnection.SetLocalDescription(offer); err != nil {
			panic(err)
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
		<-gatherComplete

		fmt.Printf("[*] Connecting to receiver at %s:%s...\n", receiverIP, receiverPort)
		conn, err := net.DialTimeout("tcp", receiverIP+":"+receiverPort, 3*time.Second)
		if err != nil {
			fmt.Printf("[-] Connection failed: %v\n", err)
			return
		}
		defer conn.Close()

		conn.Write([]byte(EncodeToString(peerConnection.LocalDescription()) + "\n"))

		reader := bufio.NewReader(conn)
		answerStr, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}

		answer := &webrtc.SessionDescription{}
		DecodeFromString(answerStr, answer)

		if err := peerConnection.SetRemoteDescription(*answer); err != nil {
			panic(err)
		}
		fmt.Println("[*] Handshake complete! Ready for chat.")
	}

	select {}
}

func setupDataChannel(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		fmt.Printf("\n=======================================================\n")
		fmt.Printf("P2P WEBRTC DATA CHANNEL ESTABLISHED!\n")
		fmt.Printf("Commands:\n")
		fmt.Printf("  - Type any message and press Enter to chat.\n")
		fmt.Printf("  - Type '/help' or '/?' for this menu.\n")
		fmt.Printf("=======================================================\n> ")
		currentLine = bytes.Buffer{} // Reset history-related state on connect
		go startTerminalChat(dc)
	})

	dc.OnClose(func() {
		fmt.Printf("\n[-] Peer disconnected.\n")
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if !msg.IsString {
			return
		}

		var m Message
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			return
		}

		switch m.Type {
		case "CHAT":
			sender := peerName
			if sender == "" && m.Sender != "" {
				sender = m.Sender
			}
			name := "[Peer]"
			if sender != "" {
				name = "[" + sender + "]"
			}
			fmt.Printf("\r%s: %s\n> ", name, m.Content)

		case "DISPLAY_NAME":
			peerName = m.Content
			fmt.Printf("\r[*] Peer's display name updated to: '%s\"\n> ", m.Content)

		case "FILE_OFFER":
			pendingOffer = &m
			waitingForAcceptance = true
			fmt.Printf("\n[Incoming File Offer]: '%s' (%d bytes)\n", m.FileName, m.FileSize)
			fmt.Printf("Do you want to accept this file? (y/n): ")

		case "FILE_ACCEPT":
			// Receiver accepted! Now transmit the actual file bytes.
			if senderFilePath == "" {
				return
			}
			data, err := os.ReadFile(senderFilePath)
			if err != nil {
				fmt.Printf("\r[Error reading file for transfer]: %v\n> ", err)
				return
			}

			dataMsg := Message{
				Type:      "FILE_DATA",
				FileName:  filepath.Base(senderFilePath),
				FileBytes: data,
			}
			payload, _ := json.Marshal(dataMsg)
			dc.SendText(string(payload))
			fmt.Printf("\r[File Sent Successfully]: '%s'\n> ", filepath.Base(senderFilePath))
			senderFilePath = ""

		case "FILE_REJECT":
			fmt.Printf("\r[File Rejected]: Peer declined '%s'.\n> ", m.FileName)
			senderFilePath = ""

		case "FILE_DATA":
			savePath := filepath.Join(defaultDownloadDir, m.FileName)
			err := os.WriteFile(savePath, m.FileBytes, 0644)
			if err != nil {
				fmt.Printf("\r[Error saving file]: %v\n> ", err)
				return
			}
			fmt.Printf("\r[File Received & Saved]: '%s' (%d bytes)\n> ", savePath, len(m.FileBytes))
		}
	})
}

func clearLine() {
	fmt.Print("\033[2K\r> ")
}

func startTerminalChat(dc *webrtc.DataChannel) {
	reader := bufio.NewReader(os.Stdin)

	for {
		// 0. Handle incoming file acceptance prompt state
		if waitingForAcceptance {
			// Wait for file acceptance (y/n) — disable arrow key history
			fmt.Print("\n> ")
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println()
				return
			}
			waitingForAcceptance = false
			text := strings.TrimSpace(line)
			answer := strings.ToLower(text)
			if answer == "y" || answer == "yes" {
				// Send ACCEPT response
				resp := Message{
					Type:     "FILE_ACCEPT",
					FileName: pendingOffer.FileName,
				}
				payload, _ := json.Marshal(resp)
				dc.SendText(string(payload))
				fmt.Printf("[*] File accepted. Waiting for transmission...\n> ")
			} else {
				// Send REJECT response
				resp := Message{
					Type:     "FILE_REJECT",
					FileName: pendingOffer.FileName,
				}
				payload, _ := json.Marshal(resp)
				dc.SendText(string(payload))
				fmt.Printf("[*] File rejected.\n> ")
			}
			pendingOffer = nil
			continue
		}

		rawInput, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Println()
			return
		}

		// --- Arrow Key Navigation (only works when currentLine is empty) ---
		isUpArrow := len(rawInput) == 3 && rawInput[0] == '\x1b' && rawInput[1] == '[' && rawInput[2] == 'A'
		isDownArrow := len(rawInput) == 3 && rawInput[0] == '\x1b' && rawInput[1] == '[' && rawInput[2] == 'B'

		if currentLine.Len() == 0 && isUpArrow {
			if histCursor > 0 {
				clearLine()
				histCursor--
				entry := history[histCursor]
				fmt.Printf(">%s\n> ", entry)
				currentLine.Reset()
				currentLine.WriteString(entry)
			}
		} else if currentLine.Len() == 0 && isDownArrow {
			if histCursor < len(history)-1 {
				clearLine()
				histCursor++
				entry := history[histCursor]
				fmt.Printf(">%s\n> ", entry)
				currentLine.Reset()
				currentLine.WriteString(entry)
			}
		} else if isUpArrow || isDownArrow {
			// Arrow pressed on non-empty line — ignore to preserve user input
		} else {
			// Normal character processing
			for _, b := range rawInput {
				if currentLine.Len() >= 1024 {
					break
				}

				// Handle escape key — skip any partial arrow sequences
				if b == '\r' || b == '\n' {
					break
				}

				currentLine.WriteByte(b)
			}
			fmt.Printf("%s\r> %s", currentLine.String(), " ")
		}

		// --- End of line processing ---
		text := strings.TrimSpace(currentLine.String())
		if text == "" {
			clearLine()
			continue
		}

		// Add to history (only if not already the last entry)
		if len(history) == 0 || history[len(history)-1] != text {
			history = append(history, text)
			if len(history) > historyMax {
				history = history[1:]
			}
			histCursor = len(history) - 1
		}

		currentLine.Reset()

		// 1. Handle /dir <path> command
		if strings.HasPrefix(text, "/dir ") {
			dirPath := strings.TrimPrefix(text, "/dir ")
			info, err := os.Stat(dirPath)
			if err != nil || !info.IsDir() {
				fmt.Printf("Invalid directory path: %s\n> ", dirPath)
				continue
			}

			defaultDownloadDir = dirPath
			fmt.Printf("Default download directory updated to: %s\n> ", defaultDownloadDir)
			continue
		}

		// 2. Handle /file <path> command
		if strings.HasPrefix(text, "/file ") {
			rawPath := strings.TrimPrefix(text, "/file ")
			info, err := os.Stat(rawPath)
			if err != nil {
				fmt.Printf("Error accessing file: %v\n> ", err)
				continue
			}

			senderFilePath = rawPath
			fileOffer := Message{
				Type:     "FILE_OFFER",
				FileName: info.Name(),
				FileSize: info.Size(),
			}
			payload, _ := json.Marshal(fileOffer)
			dc.SendText(string(payload))

			fmt.Printf("Sent file offer for '%s'. Waiting for peer approval...\n> ", info.Name())
			continue
		}

		// 3. Handle /user <name> command
		if strings.HasPrefix(text, "/user ") {
			name := strings.TrimPrefix(text, "/user ")
			displayName = name

			// Advertise our display name to the peer
			dispMsg := Message{Type: "DISPLAY_NAME", Content: displayName}
			payload, _ := json.Marshal(dispMsg)
			dc.SendText(string(payload))

			fmt.Printf("Display name set to: '%s'\n> ", displayName)
			continue
		}

		// 4. Handle /help or /? command
		if strings.HasPrefix(text, "/help") || text == "/?" {
			fmt.Printf("Current Settings:\n\n")
			if displayName != "" {
				fmt.Printf("  Display Name: %s\n", displayName)
			} else {
				fmt.Printf("  Display Name: (not set)\n")
			}
			fmt.Printf("  Download Directory: %s\n", defaultDownloadDir)
			fmt.Printf(`
Available Commands:

  /user <name>       Set your display name
                     Example: /user Alice

  /file <path>       Send a file to the connected peer
                     Example: /file ~/Documents/sample.pdf

  /dir <path>        Set download directory (defaults to current dir)
                     Example: /dir ~/Downloads/

  /help, /?          Display this help menu

> `)
			continue
		}

		// 5. Send chat message
		m := Message{
			Type:    "CHAT",
			Content: text,
		}
		if displayName != "" {
			m.Sender = displayName
		}
		payload, err := json.Marshal(m)
		if err != nil {
			fmt.Printf("Send error: %v\n> ", err)
			return
		}
		if err := dc.SendText(string(payload)); err != nil {
			fmt.Printf("Send error: %v\n> ", err)
			return
		}
	}
}

func EncodeToString(sd *webrtc.SessionDescription) string {
	b, _ := json.Marshal(sd)
	return base64.StdEncoding.EncodeToString(b)
}

func DecodeFromString(str string, sd *webrtc.SessionDescription) {
	b, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(str))
	json.Unmarshal(b, sd)
}

func printHelp() {
	fmt.Println(`
LAN P2P — Chat & File Sharing Application

USAGE
  lan-p2p [command] [arguments] [flags]

COMMANDS
  listen, host      Start listening on a local port
                    Arguments: [<PORT>] (default: 8080)
                    Example:   lan-p2p listen 9000

  connect, join     Connect to a peer node
                    Arguments: [<IP>][:<PORT>] (default: 127.0.0.1:8080)
                    Examples:  lan-p2p connect 192.168.1.12
                               lan-p2p connect :9000
                               lan-p2p connect 192.168.1.12:9000

FLAGS
  -h, --help        Show the help menu
  -v, --version     Show version details

EXAMPLES
  lan-p2p listen
  lan-p2p connect 127.0.0.1
  lan-p2p connect :8080
  `)
}

func printVersion() {
	fmt.Println(`
lan-p2p version 1.0.0
Copyright (c) 2026 Abhishek Kumar
Licensed under the MIT License
  `)
}
