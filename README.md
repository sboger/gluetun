# Gluetun — BitTorrent feature fork

> **This is a fork of [Gluetun](https://github.com/passteque/gluetun) — the
> lightweight, swiss-army-knife VPN client.** It keeps Gluetun's full feature
> set and adds a built-in BitTorrent client so you can download through the VPN
> tunnel without running a separate torrent container wired into it.

## 🔗 Original project

**[Gluetun — github.com/passteque/gluetun](https://github.com/passteque/gluetun)**

This fork is built on the upstream **[v3.41.3](https://github.com/passteque/gluetun/releases/tag/v3.41.3)**
release and is intentionally **locked at that version**. It is **not** a
contribution back upstream and is not kept in sync with later upstream
releases: it is an alternative Gluetun build that you can pin at this known
version level. Head back to the original repo for upstream issues, the wiki, and
newer releases.

## What this fork adds

Three optional features on top of stock Gluetun v3.41.3:

| Feature | Value |
| --- | --- |
| [Embedded BitTorrent client](#1-embedded-bittorrent-client) | Download torrents through the VPN tunnel, managed over the control-server API |
| [Browser web UI](#2-browser-web-ui) | A small dependency-free UI over the control-server API |
| [qBittorrent-compatible API](#3-qbittorrent-compatible-api) | Use the embedded client as a drop-in qBittorrent download client for Sonarr/Radarr |

All three are off by default — existing Gluetun configs run unchanged.

---

## 1. Embedded BitTorrent client

Enable it with `BITTORRENT_CLIENT=on`. Torrents are added, listed and removed
through the control-server API, and downloads are written to
`BITTORRENT_DOWNLOAD_DIRECTORY` (`/downloads` by default) inside the container —
always through the VPN tunnel.

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `BITTORRENT_CLIENT` | `off` | Set `on` to enable the embedded BitTorrent client. |
| `BITTORRENT_PORT` | auto | Listening port (the VPN-forwarded port if port forwarding is enabled, else a random free port). |
| `BITTORRENT_DOWNLOAD_DIRECTORY` | `/downloads` | Directory where downloaded data is stored. |
| `BITTORRENT_DHT` | `on` | Enable or disable DHT peer discovery. |
| `BITTORRENT_UPLOAD_RATE` | unlimited | Maximum upload speed in bytes/s. |
| `BITTORRENT_DOWNLOAD_RATE` | unlimited | Maximum download speed in bytes/s. |

Control-server API end points (requires the control server, and an API-key role
for full access):

```
GET    /v1/bittorrent/torrents
POST   /v1/bittorrent/torrents   {"magnet":"magnet:?xt=urn:btih:..."}
DELETE /v1/bittorrent/torrents   {"info_hash":"..."}
```

Example compose snippet:

```yml
    environment:
      - BITTORRENT_CLIENT=on
      - BITTORRENT_DOWNLOAD_DIRECTORY=/downloads
    volumes:
      - ./downloads:/downloads
```

---

## 2. Browser web UI

A small, dependency-free web UI that exposes the control-server API in the
browser (GET to view state, PUT/forms to change what the API allows). Enable it
with `GLUETUN_WEBUI=on`; it listens on `GLUETUN_WEBUI_PORT` (default `7999`) and
serves at `http://<host>:7999`.

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `GLUETUN_WEBUI` | `off` | Set `on` to enable the web UI. |
| `GLUETUN_WEBUI_PORT` | `7999` | Listening port for the web UI. |

The UI proxies `/v1/*` requests to the control server and injects its API key
automatically. To use it, configure the control server with an API-key default
role:

```yml
    environment:
      - GLUETUN_WEBUI=on
      - GLUETUN_WEBUI_PORT=7999
      - HTTP_CONTROL_SERVER_AUTH_DEFAULT_ROLE={"name":"admin","auth":"apikey","apikey":"CHANGE_ME"}
    ports:
      - "7999:7999"
```

Note: the control server denies unauthenticated requests by default, so the web
UI (and API scripts) need the API-key role above to be useful.

---

## 3. qBittorrent-compatible API

Use the embedded client as a drop-in **qBittorrent** download client. This API
speaks the qBittorrent Web API v2 subset that Sonarr/Radarr use, so you point
them at gluetun's host + port as a "qBittorrent" client and it hands the
torrents straight to the embedded BitTorrent client (downloads still exit the
VPN tunnel).

The add endpoint `POST /api/v2/torrents/add` accepts **both magnets and
`.torrent` files**:

- `urls` — comma-separated list of magnet links **or URLs to `.torrent` files**
  (each URL is fetched and parsed). This matches what Sonarr/Radarr send.
- `torrents` — multipart upload of one or more `.torrent` file binaries.

Either way the underlying client ingests torrents by magnet or by `.torrent`
bytes; non-magnet `urls` entries are fetched and added as `.torrent` files. The
older control-server `POST /v1/bittorrent/torrents` endpoint remains
magnet-only — `.torrent` support lives here.

Enable it with `QBITTORRENT_API=on`; it listens on `QBITTORRENT_API_PORT`
(default `8080`) — publish that port so Sonarr/Radarr can reach it. Leave it on
its own port (e.g. `8080`) rather than aliasing it onto the control server or
web UI ports, since the qBittorrent endpoint paths differ from the `/v1` API.

Environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `QBITTORRENT_API` | `off` | Set `on` to enable the qBittorrent-compatible API listener. |
| `QBITTORRENT_API_PORT` | `8080` | Listening port for the qBittorrent API. |
| `QBITTORRENT_API_USERNAME` | empty | Optional username. Authentication is only enforced when BOTH username and password are set. |
| `QBITTORRENT_API_PASSWORD` | empty | Optional password, paired with the username above. |

Example compose snippet:

```yml
    environment:
      - BITTORRENT_CLIENT=on
      - QBITTORRENT_API=on
      - QBITTORRENT_API_PORT=8080
      # optional auth (only enforced when both are set)
      - QBITTORRENT_API_USERNAME=CHANGE_ME
      - QBITTORRENT_API_PASSWORD=CHANGE_ME
    ports:
      - "8080:8080"
```

In Sonarr/Radarr, add a download client of type **qBittorrent** with Host set to
the gluetun host and Port `8080` (plus the username/password above if you
enabled auth). The API requires a single active Torrent client pointing at the
embedded client — it does not talk to an external qBittorrent instance.

---

## Container images

Pushing a `bittorrent-*` or `v*` tag to this repo builds and publishes a
multi-arch (amd64, arm64) container image to
**[ghcr.io/sboger/gluetun](https://github.com/sboger/gluetun/pkgs/container/gluetun)**
and creates a public GitHub release.

Current feature images:

```sh
docker pull ghcr.io/sboger/gluetun:bittorrent-v3.41.3
docker pull ghcr.io/sboger/gluetun:bittorrent-v3.41.3-qbit   # + qBittorrent API
```

Each release also carries the `latest` tag, so the newest published feature
build is available as `ghcr.io/sboger/gluetun:latest`.

## Base version

This fork is based on Gluetun **v3.41.3** — the full upstream feature set (VPN
providers, OpenVPN + WireGuard, DNS-over-TLS, firewall kill switch, built-in
HTTP/SOCKS proxies, and more) is preserved as-is, with the three features above
layered on top.
