<div align="center">

# 🤖 G-MAN

### Core Steam Network & Multi-Game Automation Framework for Go

[![Go Reference](https://img.shields.io/badge/go-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lemon4ksan/g-man)
[![Go Report Card](https://goreportcard.com/badge/github.com/lemon4ksan/g-man?style=flat-square)](https://goreportcard.com/report/github.com/lemon4ksan/g-man)
[![License](https://img.shields.io/github/license/lemon4ksan/g-man?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/lemon4ksan/g-man?style=flat-square)](https://github.com/lemon4ksan/g-man/stargazers)

> _"The right bot in the wrong place can make all the difference in the skins market."_

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

**G-man** is a high-performance, enterprise-grade Steam client SDK and multi-game automation framework architected in Go. Built for high-frequency trading, industrial-scale inventory management, and ultra-resilient network operations, G-man bridges the Steam Network and Game Coordinators into a single, thread-safe orchestrator. It seamlessly integrates **Socket (CM)**, **WebAPI**, and **Game Coordinator** protocols to keep your automation pipelines live 24/7.

```shell
go get github.com/lemon4ksan/g-man@latest
```

## 🛠 Architecture Overview

The system is designed around a decoupled, event-driven architecture using Go's CSP model. The `Client` serves as the central orchestrator, passing messages across thread-safe modules and automatically balancing workloads:

```mermaid
flowchart LR
    classDef steam fill:#1b2838,stroke:#66c0f4,stroke-width:2px,color:#fff;
    classDef transport fill:#2a475e,stroke:#66c0f4,stroke-width:1px,color:#c7d5e0;
    classDef core fill:#171a21,stroke:#cba6f7,stroke-width:2px,color:#cdd6f4;
    classDef module fill:#313244,stroke:#a6e3a1,stroke-width:1px,color:#cdd6f4;
    classDef pipeline fill:#45475a,stroke:#f9e2af,stroke-width:1px,color:#f9e2af,stroke-dasharray: 5 5;
    classDef action fill:#a6e3a1,stroke:#a6e3a1,stroke-width:2px,color:#11111b;

    subgraph External [Steam Network]
        Steam((Steam Cloud))
    end
    class External steam;

    subgraph Transport [Dual-Stack Bridge]
        direction TB
        Socket[Socket CM Client]
        WebAPI[REST / WebAPI]
    end
    class Transport,Socket,WebAPI transport;

    subgraph Core [G-MAN Orchestrator]
        Router{Service Router}
        Bus([Event Bus / Pub-Sub])
    end
    class Core,Router,Bus core;

    subgraph Modules [Domain Modules]
        direction TB
        GameGC[Game GC Dispatcher]
        Social[Chat & Friends]
        Ach[Achievements]
    end
    class Modules,GameGC,Social,Ach module;

    subgraph TradeEngine [Onion Trade Pipeline]
        direction LR
        P1[Deduplication] --> P2[Blacklist] --> P3[Price Check] --> P4[Verdicts]
    end
    class TradeEngine,P1,P2,P3,P4 pipeline;

    Verdict{Verdict}
    class Verdict action;

    Steam <--> Socket & WebAPI
    Socket & WebAPI <--> Router
    Router <--> Bus

    Bus <--> GameGC & Social & Ach

    GameGC -- "New Offer" --> P1
    P4 --> Verdict

    Verdict -- "Accept/Decline" --> Router
    Router -- "Execute" --> Steam
```

## ⚡ Key Features

### 🔄 Self-Healing Sessions (Silent Re-auth)
Downtime is lost revenue. G-man monitors the health of Web sessions and access tokens in the background. If a web cookie expires mid-request, the orchestrator automatically pauses active requests, performs an atomic OAuth2 refresh, updates the token storage, and resumes the operation transparently. Your business logic never sees a `401 Unauthorized` or standard session drop.

### 🌐 Dual-Stack Transport Engine
Stop choosing between WebAPI and Connection Manager (CM) Sockets. G-man's protocol-agnostic routing layer dynamically selects the optimal path: **TCP/WebSocket CM channels** for low-latency state synchronization, or **HTTPS WebAPI** for high-volume transactions and rate-limit mitigation. It seamlessly falls back to HTTP if a socket connection is interrupted.

### 🧅 "Onion" Trade Middleware Pipeline
Build complex trading logic as decoupled middleware layers. Process incoming trade offers through an extensible chain: `Deduplicator` $\rightarrow$ `SecurityEscrowCheck` $\rightarrow$ `BlacklistFilter` $\rightarrow$ `PriceValidator` $\rightarrow$ `Verdict`. If any middleware sets a verdict (Accept/Decline/Counter), execution halts safely, preventing race conditions.

### 🌡️ Defensive Web Scraping
Steam often throws "Soft Errors" – HTML pages returning a `200 OK` status code but displaying warning messages (e.g., "Rate Limit Exceeded", "Family View Active", or login prompts). G-man's `community` scraper scans raw response bodies, converts ambiguous HTML blocks into strictly-typed Go errors, and triggers safety handlers.

### 🛠️ Robust Dependency Management
Modules embed `module.Base` and are topologically sorted using a 3-color Depth-First Search (DFS) algorithm during boot. This ensures that modules are initialized and started in their correct topological order, detecting and failing fast on circular dependencies.

## 📂 Project Directory Structure

```text
pkg/
├── steam/            # Core Steam Protocols & Lifecycle Management
│   ├── auth/         # OAuth2 flows, persistent storage, and background token refresh
│   ├── socket/       # Stateful CM (Connection Manager) TCP/WebSocket client
│   ├── protocol/     # Steam wire-format, compiled protobufs & language specs
│   ├── transport/    # Dual-stack transport bridge (Socket/HTTP router)
│   ├── social/       # Real-time chat, persona states, and friends tracking
│   ├── community/    # Defensive scrapers (Inventories, Market, Steam Guard)
│   └── sys/          # Core subsystems (Game Coordinator dispatcher, directory)
├── behavior/         # Generic Bot Behaviors & Human Mimicry
│   └── achievements/ # Achievement simulator imitating legitimate gameplay
├── trading/          # Unified Trade Offers Engine
│   └── engine/       # Onion middleware engine with TradeContext propagation
├── bus/              # Decoupled thread-safe event bus
├── rest/             # Type-sanitizing HTTP & REST API client
├── command/          # Thread-safe command routing and processing
├── jobs/             # Asynchronous execution units and cron workers
├── crypto/           # Crypto helpers (Symmetric/Asymmetric encryption, Steam TOTP)
└── storage/          # Standard key-value storage adapters (Memory, Local JSON File)
```

## 🚀 Quick Start

### 1. Initialize the Core Client

Connect to the Steam network, authenticate, and register your automation modules in just a few lines:

```go
package main

import (
	"context"
	"os"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	webtrading "github.com/lemon4ksan/g-man/pkg/trading/web"
)

func main() {
	// 1. Set up a persistent JSON file storage for session tokens
	store, _ := jsonfile.New("storage.json")
	logger := log.New(log.DefaultConfig(log.LevelInfo))

	// 2. Instantiate the orchestrator with core modules
	config := steam.DefaultConfig()
	config.Store = store

	client, _ := steam.NewClient(config,
		steam.WithLogger(logger),
		webtrading.WithModule(webtrading.DefaultConfig()),
	)
	defer client.Close()

	// 3. Connect the Engine to the Trade Manager
	webTradeManager := webtrading.From(client)
	
	// Set up your trading engine and handler...
	// For TF2 features, you would plug in the g-man-tf2 suite here!

	// 4. Fetch optimal server and login
	dir := directory.New(client.Service())
	server, _ := dir.GetOptimalCMServer(context.Background())
	login := auth.NewLogOnDetails(os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS"))

	if err := client.Run(); err != nil {
		panic(err)
	}

	if err := client.ConnectAndLogin(context.Background(), server, login); err != nil {
		panic(err)
	}

	client.Wait()
}
```

### 2. Configure custom Onion Trading Middlewares

You can implement complex trade routing policies by building clean, decoupled middleware. Here is a custom middleware that validates the price of trade items:

```go
package main

import (
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

// PriceValidationMiddleware enforces strict price matches
func PriceValidationMiddleware(priceLimit int) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			totalGiveValue := 0
			for _, item := range ctx.Offer.ItemsToGive {
				// Access custom price metadata previously set in context by a pricer middleware
				if price, ok := ctx.Get("price_" + item.SKU); ok {
					totalGiveValue += price.(int)
				}
			}

			if totalGiveValue > priceLimit {
				// Total value exceeds our threshold: decline or send to manual review
				ctx.Review(reason.ReviewEngineError)
				return nil // Halt chain
			}

			return next(ctx)
		}
	}
}
```

## 🎮 Multi-Game Extensions

G-MAN is designed to support different games through decoupled modules. The primary supported extension is:

* **[g-man-tf2](https://github.com/lemon4ksan/g-man-tf2)**: Team Fortress 2 Trading & Economy Suite
  - **Metal Arithmetic:** Precise calculations with keys, refined, reclaimed, and scrap metals.
  - **Auto-Smelter:** Combining weapons and smelting metals dynamically to balance change.
  - **Backpack.tf Sync:** Stateful listing publisher and competitor undercutting manager.
  - **Schema Parser:** Dynamic O(1)-indexed item normalization.

## 🚀 Memory & Performance Efficiency

G-MAN is architected for maximum resource efficiency, achieving an exceptionally small runtime footprint:
- **Core Bot Architecture:** Requires only **~4.5 MB** of live heap memory (including the event bus, social modules, and trade managers).
- **Lightweight Execution:** Highly-optimized networking and strict buffer pooling minimize allocation cycles.
- **Topological Booting:** Modules are initialized deterministically, avoiding redundant locks or startup races.

## 🏗 Roadmap

### Core Infrastructure
- [x] **Smart Transport Routing:** Thread-safe dynamic requests via Sockets or HTTP.
- [x] **WebSession Keep-Alive:** Auto-refresh loops for web-cookies and API keys.
- [x] **Silent Re-Authentication:** Background recovery of expired JWTs.
- [x] **Topological Dependency Sorting:** Safe and cycle-free boot ordering of modules.
- [x] **Global Proxy Tunneling:** Clean SOCKS5/HTTP integration for all modules.
- [ ] **Steam CDN Downloader:** Dynamic downloading and parsing of app manifests/game assets.

### Trading & Protocols
- [x] **Onion Trade Middleware:** Decoupled pipelines for extensible offer checking.
- [x] **Defensive Web Scraper:** Converts soft-error HTMLs to strictly-typed errors.
- [ ] **CS2 Coordinator:** GC-handshakes, item skin parsing, and match history tracking.
- [ ] **Dota 2 Coordinator:** SOCache item parsing and custom lobby manager.

## 🤝 Contributing

We welcome contributions to G-man! If you want to add support for new storage adapters, expand GC structures, or improve defensive scraping algorithms:

1. Review our design philosophy in [CONTRIBUTING.md](CONTRIBUTING.md).
2. Ensure new network dependencies are minimal and run through the `transport.Doer` interface.
3. Write matching unit tests and verify concurrency safety using `go test -race ./...`.

## ☕ Support the Development

Building a industrial-scale Steam SDK takes hundreds of hours of protocol reverse-engineering. If G-man helped you automate your trading workflows or optimized your server resources, feel free to show some support:

<div align="center">

[![Trade Offer](https://img.shields.io/badge/Steam-Trade_Offer-blue?style=for-the-badge&logo=steam)](https://steamcommunity.com/tradeoffer/new/?partner=1141078357&token=HjsTJQFX)

> _"Donations... are not a requirement, but... they fulfill the terms of our... agreement."_

</div>

## ⚖️ Legal & License

**Disclaimer:** This software is **not** affiliated with, maintained by, or endorsed by **Valve Corporation** or any of its subsidiaries. Steam, Team Fortress 2, and all related Valve properties are registered trademarks of Valve Corporation. Use of this library is at your own risk.

This project is licensed under the **BSD 3-Clause License**. See [LICENSE](LICENSE) for full details.

---

<div align="center">
  <sub>Prepare for unforeseen consequences... or just prepare for the next Steam Sale.</sub>
</div>
