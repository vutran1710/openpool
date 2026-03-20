# Self-Hosted Media via Cloudflare Tunnel

## Status: Concept

## Core Ideas

- Users host their own media (photos, videos) on their machine
- No central media storage — zero hosting cost
- CLI serves media from local disk via Cloudflare Tunnel
- Media is ephemeral — go offline, photos disappear
- Fits the decentralized philosophy: you control your data

## How It Works

```
Viewer wants to see Alice's photos:
  1. Alice's CLI runs a local HTTP server serving ~/openpool/media/
  2. Cloudflare Tunnel (cloudflared) exposes it at a temporary URL
  3. Alice's profile includes her tunnel URL
  4. Viewer's TUI/browser fetches directly from Alice's tunnel
  5. Relay never touches media bytes
```

## Profile Integration

```yaml
# Alice's profile
media:
  - photo_1.jpg
  - photo_2.jpg
tunnel_url: https://abc123.cfargotunnel.com  # set automatically by CLI
```

## Trade-offs

| Pro | Con |
|-----|-----|
| Zero hosting cost | Requires cloudflared installed |
| User controls their data | Media unavailable when offline |
| No bandwidth on relay | ~20ms tunnel overhead (invisible for images) |
| Privacy — delete app, media gone | Setup friction for non-technical users |

## Scale

- Cloudflare Tunnel free tier: no bandwidth limits
- Each user serves their own media — scales infinitely
- Relay only routes chat ciphertext, never media

## Alternatives Considered

- Relay as media proxy — rejected (bandwidth cost on relay server)
- WebRTC peer-to-peer — rejected (NAT traversal unreliable)
- External hosting (imgur, GitHub CDN) — viable but less privacy-focused
- ASCII art avatars — fun but insufficient for real engagement

## Open

- Should CLI auto-install cloudflared or require manual setup?
- How to handle the tunnel URL lifecycle (changes on restart?)
- Fallback when viewer's terminal can't render images (link to browser?)
