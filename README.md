# WakeupService

A tiny HTTP service that runs on a Raspberry Pi and wakes your main computer via **Wake-on-LAN** (WoL). Accessible over [Tailscale](https://tailscale.com/) from your phone or any device. **ALL Security is assumed to be comming from e.g. Tailscale**

## Features

- **Web UI** — one-button page, bookmarkable to your Android home screen
- **REST endpoint** — `POST /wake` for curl / Tasker / HTTP Shortcuts automation
- Single self-contained binary, no runtime dependencies
- Config file for easy setup

---

## Quick Start

### 1. Configure

Edit `config.yaml`:

```yaml
mac_address: "AA:BB:CC:DD:EE:FF"   # ← replace with your PC's MAC address
broadcast_address: "255.255.255.255"
wol_port: 9
listen_address: ":8080"
```

**Finding your PC's MAC address:**
- Windows: `getmac /v` or `ipconfig /all`
- Linux: `ip link show`

**Finding the broadcast address** (if `255.255.255.255` doesn't work):
Use your subnet's broadcast address, e.g. `192.168.1.255` for a `/24` network.

Also make sure Wake-on-LAN is **enabled in your PC's BIOS/UEFI** and in the network adapter settings.

### 2. Build

**On the Pi itself** (if Go is installed):
```bash
go build -o wakeup-service .
```

**Cross-compile from another machine:**

| Pi model | Command |
|---|---|
| Pi 3 / 4 / 5 (64-bit) | `GOOS=linux GOARCH=arm64 go build -o wakeup-service .` |
| Pi 2 / Zero / Zero W | `GOOS=linux GOARCH=arm GOARM=7 go build -o wakeup-service .` |

Or use `make build-arm64` / `make build-armv7` if you have `make`. If you cross-compile
either rename the target executable or adjust scripts/commands in the following to 
the proper name.

### 3. Copy to Pi and run

```bash
scp wakeup-service config.yaml pi@<pi-hostname-or-ip>:~/wakeup/
ssh pi@<pi-hostname-or-ip>
cd ~/wakeup && ./wakeup-service
```

The service logs to stdout. Press `Ctrl+C` to stop.

Optionally pass a custom config path:
```bash
./wakeup-service /path/to/config.yaml
```

---

## Usage

### From Android (browser)

1. Open `http://<pi-tailscale-ip>:8080` in your browser
2. Tap the **Wake** button
3. Bookmark the page → *Add to Home Screen* for a one-tap shortcut

### REST API

```bash
curl -X POST http://<pi-tailscale-ip>:8080/wake
# → {"status":"ok"}
```

Use with [HTTP Shortcuts](https://http-shortcuts.rmy.ch/) (Android) for a home screen widget that calls the endpoint directly without opening a browser.

### From anywhere on your Tailscale network

```bash
curl -X POST http://raspberrypi:8080/wake
```

---

## Auto-start on boot (systemd)

This sets up the service to start automatically after every reboot or power failure, without needing root.

### 1. Copy the service file to the Pi

```bash
scp wakeup-service.service pi@<pi-hostname-or-ip>:~/.config/systemd/user/
```

### 2. Enable and start it

```bash
ssh pi@<pi-hostname-or-ip>

# Create the directory if it doesn't exist
mkdir -p ~/.config/systemd/user

# Reload systemd, enable on boot, and start now
systemctl --user daemon-reload
systemctl --user enable wakeup-service
systemctl --user start wakeup-service

# Allow the user service to run even when not logged in
sudo loginctl enable-linger pi
```

The `enable-linger` step is important — without it, user services only run while you're logged in via SSH.

### 3. Check status

```bash
systemctl --user status wakeup-service
journalctl --user -u wakeup-service -f   # follow live logs
```

### Useful commands

```bash
systemctl --user stop wakeup-service     # stop
systemctl --user restart wakeup-service  # restart after config change
systemctl --user disable wakeup-service  # remove from autostart
```

---

## Security

The service has no authentication. It is intentionally scoped to your **Tailscale network only** — do not expose port 8080 to the public internet.
