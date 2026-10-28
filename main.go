package main

import (
	"bufio"
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

func main() {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  Receiver: go run main.go listen")
		fmt.Println("  Initiator: go run main.go connect <RECEIVER_IP>")
		return
	}

	mode := os.Args[1]

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer peerConnection.Close()

	if mode == "listen" {
		listener, err := net.Listen("tcp", ":8080")
		if err != nil {
			panic(err)
		}
		defer listener.Close()

		fmt.Println("[*] Listening for connection on LAN port 8080...")
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

	} else if mode == "connect" {
		if len(os.Args) < 3 {
			fmt.Println("Please provide receiver IP: go run main.go connect 127.0.0.1")
			return
		}
		receiverIP := os.Args[2]

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

		fmt.Printf("[*] Connecting to receiver at %s:8080...\n", receiverIP)
		conn, err := net.DialTimeout("tcp", receiverIP+":8080", 3*time.Second)
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
		fmt.Printf("🎉 P2P WEBRTC DATA CHANNEL ESTABLISHED!\n")
		fmt.Printf("Commands:\n")
		fmt.Printf("  - Type any message and press Enter to chat.\n")
		fmt.Printf("  - Type '/help' or '/?' for this menu.\n")
		fmt.Printf("Current Download Directory: %s\n", defaultDownloadDir)
		fmt.Printf("=======================================================\n> ")
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
			fmt.Printf("[*] Peer's display name updated to: '%s'\n", m.Content)

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

func startTerminalChat(dc *webrtc.DataChannel) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())

		// 0. Handle incoming file acceptance prompt state
		if waitingForAcceptance {
			waitingForAcceptance = false
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
				fmt.Print("> ")
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
			var line string
			if displayName != "" {
				line = fmt.Sprintf("Your Display Name: %s\n", displayName)
			} else {
				line = "Your Display Name: (not set)\n"
			}
			fmt.Printf(`Available commands:
` + line + `  Type '/dir <path>' to set default download directory.
  Type '/file <path>' to send a file.
  Type '/user <name>' to set your display name.
  Type '/help' or '/?' for this menu.

> `)
			continue
		}

		if text == "" {
			fmt.Print("> ")
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
		fmt.Print("> ")
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
