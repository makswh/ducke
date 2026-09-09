# Ducke

<p align="center">
  <strong>Remote Game Repository Downloader & Steam Metadata Hub</strong><br>
  <em>Designed for Handheld & Desktop PCs — Native Steam Deck / SteamOS 3.8+ Experience</em>
</p>

<p align="center">
  <img src="build/appicon.png" alt="Ducke Icon" width="128" height="128" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%28SteamOS%29-blue" alt="Platforms" />
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/Wails-v2-DF1A2A" alt="Wails Version" />
  <img src="https://img.shields.io/badge/Frontend-Svelte%205%20%2B%20TypeScript%20%2B%20TailwindCSS-FF3E00?logo=svelte" alt="Frontend Stack" />
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License" />
</p>

---

## Highlights

- **Dual UI Modes**:
  - **Big Picture Mode**: 10-foot gamepad navigation tailored for Steam Deck, handheld consoles, and couch gaming (1280x800 native layout, sound effects, full D-pad / thumbstick control, SteamOS on-screen keyboard trigger).
  - **Desktop Mode**: High-density master-detail catalog with fast searching, filtering, and multi-pane management.
- **High-Performance Downloader**:
  - Multi-threaded chunk transfers with pause/resume support.
  - Bandwidth throttling (speed limiter) and concurrent file queuing.
  - Real-time download metrics (transfer rate, ETA, disk allocation, buffer pool recycling).
  - Protocol support: **SFTP (SSH)** and **FTP** with directory scanning and resume capability.
- **Rich Steam & SteamGridDB Metadata Integration**:
  - Automatic title cleaning and smart fuzzy matching against the Steam catalog.
  - HD screenshots and interactive video trailer previews (HLS / DASH / MP4).
  - Review scores, metacritic ratings, system requirements, genres, and release dates.
  - High-res artwork from SteamGridDB (box art, wide hero banners, logos).
- **FileZilla Import**:
  - Seamlessly import server configurations from exported `FileZilla3.xml` files.
- **SteamOS / Steam Deck Native Support**:
  - Pre-configured Flatpak packaging with hardware video acceleration (`WebviewGpuPolicyAlways`).
  - Native Gamescope & Wayland/X11 compatibility.
  - MicroSD card and external drive detection.

---

## Tech Stack

- **Backend**: [Go](https://go.dev/) 1.25, [Wails v2](https://wails.io/), [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go SQLite, zero CGO required on Windows).
- **Frontend**: [Svelte 5](https://svelte.dev/), [TypeScript](https://www.typescriptlang.org/), [Vite](https://vitejs.dev/), [TailwindCSS v4](https://tailwindcss.com/), [Lucide Svelte](https://lucide.dev/).
- **Protocols**: `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, `github.com/jlaffaye/ftp`.

---

## Project Structure

```
Ducke/
├── app.go                  # Wails application backend bindings & API bridge
├── main.go                 # Application entrypoint & window configuration
├── wails.json              # Wails project manifest
├── pkg/
│   ├── config/             # App settings, server profiles, FileZilla XML parser
│   ├── database/           # SQLite schema, games cache, download history
│   ├── downloader/         # Transfer engine, queue, rate limiter, buffer pool
│   ├── logger/             # Diagnostics logger
│   ├── metadata/           # Steam Store API & SteamGridDB fetchers, title sanitizers
│   └── remote/             # FTP and SFTP client connections and directory walkers
├── frontend/               # Svelte 5 + TypeScript + Vite frontend
│   └── src/
│       ├── lib/components/ # Views: Catalog, Detail, Downloads, Settings, Lightbox
│       │   └── bigpicture/ # 10-foot Big Picture mode UI components
│       └── lib/navigation/ # Gamepad navigation engine & sound feedback
└── build/                  # Packaging & icons (Windows NSIS, Linux Flatpak, macOS)
    └── linux/              # SteamOS scripts, Flatpak manifest, Desktop entry
```

---

## Getting Started & Development

### Prerequisites

1. **Go**: Version 1.22+ (recommended 1.24 or 1.25)
2. **Node.js**: Version 18+ and npm
3. **Wails CLI**:
   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```

### Running in Live Development Mode

```bash
# Clone the repository
git clone https://github.com/your-username/ducke.git
cd ducke

# Run with hot reload (Vite + Go live reload)
wails dev
```

---

## Building

### Windows Binary / Installer

```bash
# Compile optimized Windows executable (AMD64)
wails build -platform windows/amd64

# Output located at:
# build/bin/Ducke.exe
```

### Linux & Steam Deck (Flatpak Bundle)

To build a standalone Flatpak bundle (`Ducke.flatpak`) and native binary on Windows using WSL (Ubuntu 22.04):

```cmd
# Run the automated build script
build_linux.bat
```

Or on a Linux system with `flatpak-builder` and `webkit2gtk-4.1` installed:

```bash
chmod +x build_linux.sh
./build_linux.sh
```

The resulting files will be generated in `build/bin/`:
- `Ducke.flatpak`: Ready-to-install bundle for SteamOS / Flathub runtime.
- `Ducke`: Native Linux AMD64 binary.
- `install_steamdeck.sh`: One-click installer helper for Steam Deck Desktop Mode.

See [build/linux/README_STEAM_OS.md](build/linux/README_STEAM_OS.md) for detailed SteamOS installation and Steam Game Mode integration instructions.

---

## Running Tests

```bash
# Run all Go unit and integration tests
go test ./...

# Typecheck frontend
cd frontend
npm run check
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
