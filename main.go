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

		// ADDED: 3-second timeout so it fails gracefully instead of hanging forever
		conn, err := net.DialTimeout("tcp", receiverIP+":8080", 3*time.Second)
		if err != nil {
			fmt.Printf("[-] Connection failed or timed out: %v\n", err)
			fmt.Println("[-] Hint: If testing locally on the same Mac, use '127.0.0.1' instead of your Wi-Fi IP.")
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

type FileMessage struct {
	FileName  string `json:"fileName"`
	FileBytes []byte `json:"fileBytes"`
}

func setupDataChannel(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		fmt.Printf("\n========================================\n")
		fmt.Printf("🎉 P2P WEBRTC DATA CHANNEL ESTABLISHED!\n")
		fmt.Printf("Type your message and press Enter to chat.\n")
		fmt.Printf("Type '/file <path>' to send a file.\n")
		fmt.Printf("========================================\n> ")
		go startTerminalChat(dc)
	})

	dc.OnClose(func() {
		fmt.Printf("\n[-] Peer disconnected.\n")
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			var fileMsg FileMessage
			err := json.Unmarshal([]byte(msg.Data), &fileMsg)
			if err == nil && fileMsg.FileName != "" {
				// Automatically save incoming files to current directory to prevent stdin blocking bugs
				savePath := fileMsg.FileName
				err = os.WriteFile(savePath, fileMsg.FileBytes, 0644)
				if err != nil {
					fmt.Printf("\r[Error saving file]: %v\n> ", err)
					return
				}
				fmt.Printf("\r[File Received & Saved]: '%s' (%d bytes)\n> ", savePath, len(fileMsg.FileBytes))
			} else {
				fmt.Printf("\r[Peer]: %s\n> ", string(msg.Data))
			}
		} else {
			err := os.WriteFile("received_file.bin", msg.Data, 0644)
			if err != nil {
				fmt.Printf("\r[Error saving file]: %v\n> ", err)
				return
			}
			fmt.Printf("\r[File Received]: saved as 'received_file.bin' (%d bytes)\n> ", len(msg.Data))
		}
	})
}

func startTerminalChat(dc *webrtc.DataChannel) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "/file ") {
			rawPath := strings.TrimPrefix(text, "/file ")
			rawPath = strings.TrimSpace(rawPath)
			rawPath = strings.Trim(rawPath, "\"'")

			data, err := os.ReadFile(rawPath)
			if err != nil {
				fmt.Printf("Error reading file: %v\n> ", err)
				fmt.Print("> ")
				continue
			}

			fileName := filepath.Base(rawPath)

			fileMsg := FileMessage{
				FileName:  fileName,
				FileBytes: data,
			}
			jsonBytes, _ := json.Marshal(fileMsg)

			err = dc.SendText(string(jsonBytes))
			if err != nil {
				fmt.Printf("Error sending file: %v\n> ", err)
				fmt.Print("> ")
				continue
			}

			fmt.Printf("File '%s' sent successfully!\n> ", fileName)
			continue
		}

		if text == "" {
			fmt.Print("> ")
			continue
		}

		err := dc.SendText(text)
		if err != nil {
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
