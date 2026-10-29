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

## Launching the Application

### CLI Flags

| Flag | Description |
|------|-------------|
| `-h, --help` | Display the full help menu with usage and examples |
| `-v, --version` | Show version details |

**Help Menu:**

```text
Usage:
  lan-p2p [command] [arguments] [flags]

COMMANDS
  listen, host      Start listening on a local port
                    Arguments: [<PORT>] (default: 8080)
                    Example:   lan-p2p listen 9000

  connect, join     Connect to a peer node
                    Arguments: [<IP>][:<PORT>]
                    Defaults to: 127.0.0.1:8080
                    Examples:
                      lan-p2p connect 192.168.1.12
                      lan-p2p connect :9000
                      lan-p2p connect 192.168.1.12:9000

FLAGS
  -h, --help        Show the help menu
  -v, --version     Show version details
```

### Mode 1: Testing on the Same Machine (Two Terminal Windows)

1. **Terminal 1 (Receiver/Listener):**
   ```bash
   ./lan-p2p listen
   # or with a custom port:
   ./lan-p2p listen 9000
   ```

2. **Terminal 2 (Initiator/Connector):**
   ```bash
   ./lan-p2p connect 127.0.0.1
   ```

### Mode 2: Testing Across Two Separate Devices on the Same LAN

1. **Find the Receiver's Local IP:**
   - **macOS/Linux:** `ifconfig` or `ip a` (e.g., `192.168.1.16`)
   - **Windows:** `ipconfig`

2. **On Device A (Receiver):**
   ```bash
   ./lan-p2p listen 8080
   # or use the alias:
   ./lan-p2p host 8080
   ```

3. **On Device B (Initiator):**
   ```bash
   ./lan-p2p connect 192.168.1.16
   # or with a custom port:
   ./lan-p2p connect 192.168.1.16:9000
   # or use the alias:
   ./lan-p2p join 192.168.1.16:9000
   ```

---

## Commands During a Session

Once connected, a confirmation header will appear:

```text
=======================================================
P2P WEBRTC DATA CHANNEL ESTABLISHED!
Commands:
  - Type any message and press Enter to chat.
  - Type '/help' or '/?' for this menu.
=======================================================
> 
```

### 1. Interactive Chat & Input History

Type any text and press **Enter** to send a real-time message to your peer. If you've set a display name with `/user`, peers will see your name in the message:

```text
> Hello from Terminal 1!
[Alice]: Hey there!
```

**Input History (Up/Down Arrows):** Every non-command message you send is saved to an in-session history. When the prompt is blank, press **↑ Up Arrow** to recall your previous messages and **↓ Down Arrow** to navigate forward. This avoids re-typing the same command or message:

```text
> How's it going?                           ← sent, saved to history
  [Peer]: Fine thanks!
>                                        ← prompt is blank...
  (↑ Up)                                  ...press up arrow...
> How's it going?                         ← previous entry recalled instantly
  (Press Enter to resend or edit first)   ...edit or submit
```

### 2. Settings & Help Menu

At any time during a session, press `/help` or `/?` to see your current settings and all available commands:

```text
> /help
Current Settings:

  Display Name: (not set)
  Download Directory: .

Available Commands:

  /user <name>       Set your display name
                     Example: /user Alice

  /file <path>       Send a file to the connected peer
                     Example: /file ~/Documents/sample.pdf

  /dir <path>        Set download directory (defaults to current dir)
                     Example: /dir ~/Downloads/

  /help, /?          Display this help menu
```

### 3. Set Your Display Name

Use `/user <name>` to set an identity that peers see instead of the generic `[Peer]`:

```text
> /user Alice
Display name set to: 'Alice'
[*] Peer's display name updated to: 'Bob"
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

> /file "/Users/abhishekkumar/Documents/sample.pdf"
Sent file offer for 'sample.pdf'. Waiting for peer approval...
```

### 6. Accept / Reject Incoming Files

When a peer offers a file, you will be prompted to accept or decline:

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
