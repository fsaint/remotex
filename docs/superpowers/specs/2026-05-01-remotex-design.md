# RemoteX — Design Spec
**Date:** 2026-05-01
**Status:** Approved

## Summary

RemoteX is a system for creating named terminal sessions on a Mac and accessing them transparently from an iPhone. Each tmux session has an associated mosh-server, providing persistent, roaming-resilient connections over a Tailscale private network. Authentication is established once via QR code pairing; all subsequent connections are seamless.

---

## Architecture

Three components communicating over Tailscale:

```
Mac                                    iPhone
┌─────────────────────────┐           ┌──────────────────────┐
│  remotex CLI            │           │  RemoteX iOS App     │
│  - creates tmux session │           │  - lists sessions    │
│  - spawns mosh-server   │           │  - taps to connect   │
│  - registers w/ daemon  │           │  - terminal UI       │
│                         │           │  - libmosh embedded  │
│  remotex-daemon         │◄──REST────│                      │
│  - launchd service      │           │                      │
│  - tracks sessions      │◄──mosh────│                      │
│  - serves /sessions API │  (direct) │                      │
└─────────────────────────┘           └──────────────────────┘
```

**Connection flow:**
1. `remotex new "work"` on Mac → tmux session created, mosh-server spawned on a random UDP port, daemon records the session
2. iPhone app fetches `http://mac.tailnet:7654/sessions` → receives session list with ports
3. Tap session → app connects via libmosh → lands inside the tmux session
4. Network change (WiFi→cell) → mosh reconnects silently, session uninterrupted

---

## Mac Components

### `remotex` CLI

**Language:** Go (single binary, easy distribution)

**Commands:**
```
remotex setup             # first-time: generate keys, start daemon, show QR code
remotex new <name>        # create tmux session + start mosh-server + register with daemon
remotex list              # show active sessions, ports, uptime
remotex kill <name>       # kill tmux session + mosh-server + unregister from daemon
```

**`remotex new` internals:**
1. `tmux new-session -d -s <name>`
2. Start `mosh-server` on a random available UDP port, capturing the port and key it prints
3. `POST http://localhost:7654/internal/sessions` with `{name, port, tmux_pid, mosh_pid, started_at}`

### `remotex-daemon`

**Language:** Go
**Managed by:** launchd (`~/Library/LaunchAgents/com.remotex.daemon.plist`)
**Session store:** `~/.remotex/sessions.json`

**APIs:**

| Interface | Scope | Endpoints |
|-----------|-------|-----------|
| Internal | localhost only | `POST /internal/sessions`, `DELETE /internal/sessions/:name` |
| External | Tailscale interface, port 7654 | `GET /sessions` (API key required) |

**Watchdog:** polls tmux + mosh pids every 30 seconds, prunes dead sessions from the registry automatically.

**On startup:** reads `sessions.json`, checks all pids, removes stale entries.

---

## iOS App

**Language:** Swift + SwiftUI
**Terminal backend:** libmosh compiled as xcframework (sourced from Blink Shell open source)

### Screens

**Sessions screen** (main)
- Loads session list from daemon on appear + pull-to-refresh
- Shows: session name, status (live / dead), time started
- Dead sessions shown greyed out
- Tap live session → connect

**Terminal screen**
- Full-screen terminal emulator backed by libmosh
- Hardware keyboard supported
- Correct `SIGWINCH` sent on orientation change
- Swipe down to disconnect

**Setup screen** (first launch only)
- QR code scanner
- Stores `{tailscale_hostname, api_key, ssh_private_key}` in iOS Keychain
- Never shown again after successful pairing

### v1 Scope
- Sessions are created from the Mac CLI only (`remotex new`)
- iOS app is read + connect only (no session creation from phone in v1)

---

## Auth & Pairing

### One-time setup (`remotex setup` on Mac)
1. Generates Ed25519 SSH keypair → `~/.remotex/id_ed25519`
2. Appends public key to `~/.ssh/authorized_keys`
3. Generates a random 256-bit API key
4. Auto-detects Tailscale hostname via `tailscale status --json`
5. Encodes `{host, api_key, ssh_private_key}` as a QR code, printed to terminal
6. Starts the daemon launchd service

### iPhone pairing
1. Open RemoteX → tap "Pair with Mac"
2. Scan QR code
3. Credentials stored in iOS Keychain
4. Done — no passwords ever again

### Ongoing auth
- All REST calls include `Authorization: Bearer <api_key>`
- mosh uses the SSH keypair for initial handshake; mosh's UDP encryption handles the rest
- Daemon binds exclusively to the Tailscale network interface — not reachable from the public internet

### Key rotation
- Re-running `remotex setup` rotates the API key and keypair
- Re-pairing the iPhone is required after rotation

---

## Error Handling & Edge Cases

### Network
| Scenario | Behavior |
|----------|----------|
| WiFi → cell mid-session | mosh reconnects transparently, no user action |
| Tailscale down | iOS app shows "Cannot reach Mac" with retry button |
| Mac asleep | mosh suspends; resumes when Mac wakes |

### Session lifecycle
| Scenario | Behavior |
|----------|----------|
| Mac rebooted | launchd restarts daemon; startup prunes dead pids from sessions.json |
| tmux session manually killed | watchdog detects within 30s; iOS app shows session greyed out on next refresh |
| mosh-server dies, tmux lives | daemon detects and attempts to respawn mosh-server for that session |

### iOS app
| Scenario | Behavior |
|----------|----------|
| App backgrounded during session | mosh connection suspends, resumes on foreground |
| Screen rotated / font size changed | terminal sends SIGWINCH to remote, layout reflows |
| QR code scanned twice | API key remains valid (idempotent), no issue |

---

## Out of Scope (v1)
- Creating sessions from the iPhone
- Session sharing between multiple users
- Android app
- Web UI fallback
- Session recording / replay
