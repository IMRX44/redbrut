# redbrut

High-performance RDP credential testing tool written in Go.

- **Linux:** Interactive TUI — form-based config + live monitoring dashboard
- **Windows:** Native dark-themed GUI with file pickers and live stats

---

## Why redbrut over NLBrute

| | NLBrute | redbrut |
|---|---|---|
| Concurrency | 500 OS threads (~1MB each) | 5,000+ goroutines (~2KB each) |
| Protocol | Full RDP GUI negotiation | NLA-only handshake (faster) |
| Unicode passwords | ❌ No Cyrillic/Chinese | ✅ UTF-8 → UTF-16LE (full NTLMv2) |
| False negatives | Possible | ❌ Never — NTSTATUS-based classification |
| Scale | Limited | 100,000+ IP lists (streaming, no RAM overflow) |
| Platform | Windows only | Linux + Windows |

---

## Features

- **Custom NLA/CredSSP handshake** — pure Go, no FreeRDP dependency
- **NTLMv2 with proper UTF-16LE encoding** — Russian, Chinese, Arabic passwords work correctly
- **NTSTATUS result classification** — distinguishes invalid, locked, expired, network errors
- **No false negatives** — a credential is only marked invalid when the server explicitly says so
- **Per-IP rate limiting** — token bucket prevents account lockout
- **Auto-pause on lockout** — stops targeting a user/IP when `STATUS_ACCOUNT_LOCKED_OUT` received
- **Resume support** — Ctrl+C any time, continue with `--resume`
- **Password spray mode** — one password across all targets before moving to next
- **Streaming combo generator** — 100k IPs × 1k users × 10k passwords without RAM issues

---

## Usage

### Linux (TUI)

```bash
./redbrut-linux
```

The TUI will prompt you for all settings interactively:

```
┌─ Step 1: Files ──────────────────────────┐
│  Targets File    targets.txt             │
│  Users File      users.txt               │
│  Passwords File  passwords.txt           │
│  Output File     goods.txt               │
└──────────────────────────────────────────┘

┌─ Step 2: Settings ───────────────────────┐
│  Concurrency     5000                    │
│  Rate / IP / s   5                       │
│  Timeout (s)     5                       │
│  Attack Mode     Credential              │
│  Resume?         No                      │
└──────────────────────────────────────────┘
```

Live monitor:
```
 redbrut  ▸  RDP Credential Testing
 Progress: 4521 / 50000 (9.0%)  elapsed: 14s
 [████████░░░░░░░░░░░░░░░░░░░░░░]

  ✓ Found: 3    ⊘ Locked: 2    ✗ Errors: 14    ↻ Retry: 8    ⚡ 312 req/s

  ──────────────────────────────────────────────────────
  [+] 45.67.89.10:3389    admin:Пароль123       SUCCESS
  [+] 192.168.1.50:3389   user:密码2024         SUCCESS
  [!] 10.0.0.5:3389       administrator         LOCKED
```

### Windows (GUI)

Run `redbrut-windows.exe` — a dark-themed window opens with two tabs:

- **Config tab:** Browse files, set concurrency/rate/timeout, choose mode
- **Live tab:** Real-time progress bar, speed, found/locked/error counters, scrollable log

---

## Input Files

**targets.txt** — one `IP:PORT` per line (port defaults to 3389):
```
192.168.1.10:3389
10.0.0.5
45.67.89.100:3390
```

**users.txt** — one username per line:
```
administrator
admin
user
DOMAIN\user
```

**passwords.txt** — one password per line, UTF-8 encoded:
```
Password123
Пароль2024
密码123456
Summer@2024
```

---

## Output

**goods.txt** — valid credentials, one per line:
```
192.168.1.10:3389    administrator    Password123
45.67.89.10:3389     admin            Пароль2024
```

**goods.txt.resume** — session state for `--resume` (auto-managed)

---

## How It Works

### NLA Handshake

Instead of spawning a full RDP session, redbrut performs only the NLA (Network Level Authentication) handshake:

```
TCP connect → TLS upgrade → CredSSP NEGOTIATE → Server CHALLENGE
→ NTLM AUTHENTICATE (NTLMv2) → Read TSRequest → Parse NTSTATUS
```

This stops at the authentication layer — no GUI negotiation, no screen rendering. Each attempt takes ~100–200ms of pure network time.

### Result Classification

| NTSTATUS Code | Meaning | Action |
|---|---|---|
| `0x00000000` | SUCCESS | Save to goods.txt |
| `0xC000006D` | LOGON_FAILURE | Skip (invalid) |
| `0xC0000234` | ACCOUNT_LOCKED | Pause IP 30min |
| `0xC0000071` | PASSWORD_EXPIRED | Save (valid!) |
| TCP timeout / refused | Network error | Retry with backoff |
| Anything else | Unknown | Retry — never mark invalid |

### Unicode Support

NTLMv2 hashes are computed over `UTF-16LE(password)`. redbrut converts UTF-8 input → `unicode/utf16` → little-endian bytes before hashing, so any Unicode password works correctly.

---

## Build from Source

```bash
git clone https://github.com/imrx44/redbrut
cd redbrut

# Linux
go build -ldflags="-s -w" -o redbrut ./cmd/redbrut-linux/

# Windows (requires MinGW-w64 or MSYS2 on the build machine)
GOOS=windows go build -ldflags="-s -w" -o redbrut.exe ./cmd/redbrut-windows/
```

Requirements: Go 1.21+. Windows build requires a C compiler (MinGW) for fyne.

---

## Legal

This tool is for **authorized penetration testing and red team engagements only**. Only use against systems you own or have explicit written permission to test. Unauthorized use is illegal.
