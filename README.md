# redbrut — RDP Brute Force Tool for Penetration Testing

**redbrut** is a high-performance RDP credential testing tool written in Go. It implements the NLA/CredSSP handshake from scratch — no FreeRDP, no dependencies — and classifies every result by NTSTATUS code, making it the fastest and most accurate open-source RDP brute-forcing tool available.

- **Linux:** Interactive TUI — form-based config + live monitoring dashboard
- **Windows:** Native dark-themed GUI with file pickers and real-time stats

---

## Why redbrut

| | NLBrute | redbrut |
|---|---|---|
| Concurrency | 500 OS threads (~1 MB each) | 5,000+ goroutines (~2 KB each) |
| Protocol | Full RDP GUI negotiation | NLA-only handshake (3× faster) |
| Unicode passwords | ❌ Breaks on Cyrillic/Chinese | ✅ Full NTLMv2 over UTF-16LE |
| False negatives | Possible | ❌ Never — NTSTATUS classification |
| Target scale | Limited | 100,000+ IPs, streaming (no RAM cap) |
| Lockout protection | Manual | ✅ Auto-pause per IP |
| Platform | Windows only | Linux + Windows |
| Resume | ❌ | ✅ Ctrl+C and `--resume` |

---

## Features

- **Pure Go NLA/CredSSP** — no FreeRDP, no libssl, single static binary
- **NTLMv2 with UTF-16LE encoding** — Russian, Chinese, Arabic, emoji passwords all work
- **NTSTATUS result classification** — zero false negatives, distinguishes invalid / locked / expired / network error
- **Per-IP token-bucket rate limiting** — configurable req/s per host
- **Auto-pause on lockout** — stops targeting an IP the moment `STATUS_ACCOUNT_LOCKED_OUT` arrives
- **Resume support** — interrupt with Ctrl+C at any point, resume from exact position
- **Password spray mode** — one password across all targets before cycling to the next
- **Streaming combo generator** — 100k IPs × 1k users × 10k passwords, constant memory usage
- **Exponential retry backoff** — network errors retried up to configurable limit (2s → 4s → 8s → 16s cap)

---

## Download

Go to the [Releases](../../releases) page and download the binary for your platform:

| File | Platform |
|------|----------|
| `redbrut-linux-amd64` | Linux x86\_64 |
| `redbrut-linux-arm64` | Linux ARM64 (servers, VPS) |
| `redbrut-windows-amd64.exe` | Windows x86\_64 |

```bash
# Linux — make executable and run
chmod +x redbrut-linux-amd64
./redbrut-linux-amd64
```

---

## Usage

### Linux (TUI)

Launch the binary. The TUI walks you through config step by step, then switches to a live monitoring dashboard.

**Step 1 — Files:**
```
┌─────────────────────────────────────────────┐
│  Targets File    targets.txt                │
│  Users File      users.txt                  │
│  Passwords File  passwords.txt              │
│  Output File     goods.txt                  │
└─────────────────────────────────────────────┘
```

**Step 2 — Settings:**
```
┌─────────────────────────────────────────────┐
│  Concurrency     5000                       │
│  Rate / IP / s   5                          │
│  Timeout (s)     5                          │
│  Attack Mode     Credential Stuffing        │
│  Resume?         No                         │
└─────────────────────────────────────────────┘
```

**Live monitor:**
```
 redbrut  ▸  RDP Credential Testing
 Progress: 4521 / 50000 (9.0%)  elapsed: 14s
 [████████░░░░░░░░░░░░░░░░░░░░░░]

  ✓ Found: 3    ⊘ Locked: 2    ✗ Errors: 14    ↻ Retry: 8    ⚡ 312 req/s

  ─────────────────────────────────────────────────────────────
  [+] 45.67.89.10:3389    admin:Пароль123       SUCCESS
  [+] 192.168.1.50:3389   user:密码2024         SUCCESS
  [!] 10.0.0.5:3389       administrator         LOCKED (paused 30m)
```

### Windows (GUI)

Run `redbrut-windows-amd64.exe`. A dark-themed window opens with two tabs:

- **Config** — browse for target/user/password files, set concurrency, rate, timeout, attack mode
- **Live** — real-time progress bar, req/s counter, found/locked/error stats, scrollable credential log

---

## Input Files

**targets.txt** — one `IP:PORT` per line (port defaults to 3389 if omitted):
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
DOMAIN\svcaccount
```

**passwords.txt** — one password per line, UTF-8 encoded:
```
Password123
Пароль2024
密码123456
Summer@2024!
```

---

## Output

**goods.txt** — confirmed valid credentials, tab-separated:
```
192.168.1.10:3389    administrator    Password123
45.67.89.10:3389     admin            Пароль2024
```

**goods.txt.resume** — session checkpoint file, managed automatically. Delete to start fresh.

---

## How It Works

### NLA/CredSSP Handshake

redbrut speaks only the authentication layer of RDP — it never negotiates a display or sends input events. Each attempt follows this path:

```
TCP connect
  → TLS upgrade (starttls)
  → CredSSP NEGOTIATE token
  → Server NTLM CHALLENGE
  → NTLMv2 AUTHENTICATE (HMAC-MD5 over UTF-16LE password)
  → Read TSRequest response
  → Parse NTSTATUS code
  → Close connection
```

Total wire time per attempt: ~100–200 ms on LAN, ~300–500 ms on WAN. No screen rendering, no clipboard negotiation, no virtual channel overhead.

### NTSTATUS Classification

| Code | Meaning | Action |
|------|---------|--------|
| `0x00000000` | `STATUS_SUCCESS` | Save to output |
| `0xC0000064` | `STATUS_NO_SUCH_USER` | Invalid — skip |
| `0xC000006D` | `STATUS_LOGON_FAILURE` | Invalid — skip |
| `0xC000006E` | `STATUS_ACCOUNT_RESTRICTION` | Invalid — skip |
| `0xC0000234` | `STATUS_ACCOUNT_LOCKED_OUT` | Pause IP 30 min |
| `0xC0000071` | `STATUS_PASSWORD_EXPIRED` | Save (login still works) |
| `0xC000006F` | `STATUS_INVALID_LOGON_HOURS` | Invalid — skip |
| TCP error / TLS error | Network issue | Retry with backoff |
| Any other code | Unknown | Retry — never mark invalid |

### Unicode Password Support

NTLMv2 requires the password as `UTF-16LE` bytes before hashing. Most tools pass raw bytes of the UTF-8 string, which produces the wrong hash for any non-ASCII character. redbrut uses Go's `unicode/utf16` package to convert correctly, so passwords in any script hash identically to how Windows computes them.

---

## Build from Source

```bash
git clone https://github.com/imrx44/redbrut
cd redbrut

# Linux — pure Go, no C compiler needed
go build -ldflags="-s -w" -o redbrut ./cmd/redbrut-linux/

# Windows — requires CGo + MinGW-w64 for the fyne GUI
# On Windows with MSYS2:
set CGO_ENABLED=1
go build -ldflags="-s -w -H windowsgui" -o redbrut.exe ./cmd/redbrut-windows/
```

Requirements: Go 1.21+. Windows GUI requires a C compiler (MinGW-w64 / MSYS2).

---

## Legal

This tool is for **authorized penetration testing and red team engagements only**.  
Only use against systems you own or have explicit written permission to test.  
Unauthorized use violates computer crime laws in most jurisdictions.
