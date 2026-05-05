# RemoteX

Access your Mac terminal sessions from iPhone over Tailscale.

## What it does

RemoteX lets you connect to persistent tmux sessions running on your Mac directly from your iPhone. It uses mosh for reliable transport, SSH for authentication, and Tailscale for networking — no port forwarding or public IPs required.

## Architecture

```
iPhone (iOS app)  ──mosh/Tailscale──>  Mac (remotex-daemon)
                                             │
                                          tmux sessions
```

- **Mac side:** Go daemon (`remotex-daemon`) manages tmux sessions and spawns mosh-server on demand. A CLI (`remotex`) controls the daemon.
- **iOS side:** SwiftUI app with libmosh + SwiftTerm renders a full terminal.
- **Auth:** Ed25519 SSH keypair + API key, shared once via QR code scan, stored in iOS Keychain.
- **Network:** Tailscale CGNAT (100.x.x.x) — daemon binds only to the Tailscale interface.

## Structure

```
mac/
  cmd/remotex/          # CLI
  cmd/remotex-daemon/   # HTTP daemon (port 7654)
  internal/config/      # ~/.remotex/config.json
  internal/session/     # Session manager + watchdog
  internal/tmux/        # tmux CLI wrapper
  internal/mosh/        # mosh-server spawn/parse
  internal/daemon/      # HTTP server + handlers
  internal/setup/       # Key generation, QR code, launchd

ios/
  RemoteX/App/          # AppRouter, entry point
  RemoteX/Models/       # Session, Credentials, ConnectInfo
  RemoteX/Services/     # KeychainStore, DaemonClient, MoshSession
  RemoteX/Screens/      # SetupView, SessionsView, TerminalView
  RemoteX/Utilities/    # TerminalSizeHelper
  Frameworks/           # libmosh.xcframework, Protobuf_C_.xcframework (not in git — obtain separately)
```

## Requirements

- Mac: Go 1.22+, tmux, mosh-server, Tailscale
- iPhone: iOS 16+, Tailscale
- Both devices on the same Tailscale network

## Setup

```bash
# Mac — build and install
cd mac && go build ./cmd/remotex ./cmd/remotex-daemon
./remotex setup   # generates keys, prints QR code, installs launchd service

# iOS — open in Xcode, build & run, scan QR code on first launch
```
