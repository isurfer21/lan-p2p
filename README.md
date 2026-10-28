# LAN P2P - Chat & File Sharing Application

A lightweight, terminal-based peer-to-peer (P2P) chat and file-sharing tool written in Go. Built using **Pion WebRTC**, this application creates encrypted, direct connections between devices on the same Local Area Network (LAN) without requiring web browsers, third-party cloud servers, or external internet access.

---

## Features

- **Pure CLI Experience:** Complete terminal interface for messaging and file transfers.
- **Direct P2P WebRTC Connection:** Encrypted data transfer directly between peers using `pion/webrtc`.
- **Zero-Paste Automated Handshake:** Peer discovery and signaling handled over local TCP sockets without copying/pasting Base64 strings.
- **Interactive File Transfer Protocol:**
  - Lightweight file offer sent prior to data transfer.
  - Recipient approval (`y/n`) prompt before file bytes are transmitted.
  - Original filename and file size preservation.
- **Display Names:** Use `/user <name>` to set your identity — peers see `[YourName]: ...` instead of the generic `[Peer]`.
- **Commands & Help:** Press `/help` or `/?` at any time for a full command reference.
- **Cross-Platform:** Runs on macOS, Linux, and Windows.

---

## Prerequisites

- **Go:** Version 1.20 or later installed on your system.
- **Local Network:** Both devices must be connected to the same LAN / Subnet (or test on the same machine using `127.0.0.1`).

---

## Installation & Setup

1. **Clone or download the project folder:**
   ```bash
   git clone https://github.com/isurfer21/lan-p2p.git
   cd lan-p2p
   ```

2. **Initialize modules and install dependencies:**
   ```bash
   go mod init lan-p2p
   go get github.com/pion/webrtc/v4
   ```

3. **Build the binary:**
   ```bash
   go build -o lan-p2p
   ```

---

## How to Run

### Mode 1: Testing on the Same Machine (Two Terminal Windows)

1. **Terminal 1 (Receiver/Listener):**
   ```bash
   ./lan-p2p listen
   ```

2. **Terminal 2 (Initiator/Connector):**
   ```bash
   ./lan-p2p connect 127.0.0.1
   ```

---

### Mode 2: Testing Across Two Separate Devices on the Same LAN

1. **Find the Receiver's Local IP:**
   - **macOS/Linux:** `ifconfig` or `ip a` (e.g., `192.168.1.16`)
   - **Windows:** `ipconfig`

2. **On Device A (Receiver):**
   ```bash
   ./lan-p2p listen
   ```

3. **On Device B (Initiator):**
   ```bash
   ./lan-p2p connect 192.168.1.16
   ```

---

## Usage Guide & Commands

Once connected, a confirmation header will appear:

```text
=======================================================
🎉 P2P WEBRTC DATA CHANNEL ESTABLISHED!
Commands:
  - Type any message and press Enter to chat.
  - Type '/help' or '/?' for this menu.
Current Download Directory: .
=======================================================
> 
```

### 1. Interactive Chat

Type any text and press **Enter** to send a real-time message to your peer. If you've set a display name with `/user`, peers will see your name in the message:

```text
> Hello from Terminal 1!
[Alice]: Hey there!
```

### 2. Help Menu

At any time during a session, press `/help` or `/?` for a full command reference:

```text
> /help
Available commands:
Your Display Name: (not set)
  Type '/dir <path>' to set default download directory.
  Type '/file <path>' to send a file.
  Type '/user <name>' to set your display name.
  Type '/help' or '/?' for this menu.

> 
```

### 3. Set Your Display Name

Use `/user <name>` to set an identity that peers see instead of the generic `[Peer]`:

```text
> /user Alice
Display name set to: 'Alice'
[*] Peer's display name updated to: 'Bob'
> Hey Bob!
[Alice]: Hi Alice!
```

### 4. Configure Download Directory
By default, received files save to the current working directory (`.`). You can change this at any time:
```text
> /dir /Users/abhishekkumar/Downloads
Default download directory updated to: /Users/abhishekkumar/Downloads
```

### 5. Send a File
Use `/file` followed by the absolute or relative file path (quotes and spaces are automatically trimmed):
```text
> /file "/Users/abhishekkumar/Documents/sample.pdf"
Sent file offer for 'sample.pdf'. Waiting for peer approval...
```

### 6. Accept / Reject Incoming Files
When a peer offers a file, you will be prompted to accept or decline:
```text
[Incoming File Offer]: 'sample.pdf' (1048576 bytes)
Do you want to accept this file? (y/n): y

[*] File accepted. Waiting for transmission...
[File Received & Saved]: 'sample.pdf' (1048576 bytes)
```

---

## Troubleshooting

- **Connection Hangups / Firewalls:**
  - If connecting across devices times out, verify that your local OS firewall allows incoming connections on port `8080`.
  - When running both instances on the same machine, always connect via `127.0.0.1` rather than your Wi-Fi interface IP to avoid OS loopback blocking.
- **Port 8080 in use:**
  - If port `8080` is busy, ensure no previous instance is running (`lsof -i :8080` or `netstat -aon | grep 8080`).

---

## License

Distributed under the [MIT License](LICENSE).
