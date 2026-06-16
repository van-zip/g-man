<div align="center">

# ⚙️ Steam Core Orchestrator

### Main Client Lifecycle, Protocol Routing, and Event Dispatching

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

The `steam` package acts as the central coordinator of G-man. It unifies low-latency binary CM socket protocols with stateless WebAPI and Steam Community scrapers into a single, thread-safe `steam.Client`.

## ⚙️ Core Responsibilities

* **Unified Protocol Routing:** Exposes a single `Client.Do()` interface. If the client is connected to a CM socket, requests are serialized directly into binary `EMsg` packets. If disconnected or if the destination requires HTTPS, requests route through the WebAPI client.
* **Token State Management:** Coordinates the authorization cycle (OAuth2 flows, Web Cookie generation, API keys, and Steam Guard handshakes) and automatically handles refreshes to maintain connection continuity.
* **Event Dispatching:** Integrates directly with the central thread-safe event bus to publish state changes (e.g. connection losses, successful auth events, or incoming raw socket packets) to registered modules.

## 🚀 Usage Example: Authenticating and Executing RPCs

The example below demonstrates how to initialize the client, connect to the Steam Network, authenticate, and call a Unified Service API using proto-compiled structures:

```go
package main

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx := context.Background()
	logger := log.New(log.DefaultConfig(log.LevelInfo))
	defer logger.Close()

	// 1. Configure and instantiate the Orchestrator
	cfg := steam.DefaultConfig()
	client, err := steam.NewClient(cfg, steam.WithLogger(logger))
	if err != nil {
		logger.Error("Orchestrator initialization failed", log.Err(err))
		return
	}
	defer func() {
		_ = client.Close()
		client.Wait()
	}()

	// Start internal loops
	if err := client.Run(); err != nil {
		panic(err)
	}

	// 2. Fetch optimal connection servers from directory
	dir := directory.New(client.Service())
	server, err := dir.GetOptimalCMServer(ctx)
	if err != nil {
		logger.Error("Server resolution failed", log.Err(err))
		return
	}

	// 3. Authenticate with credentials
	details := &auth.LogOnDetails{
		AccountName: "your_username",
		Password:    "your_password",
	}

	logger.Info("Connecting to Steam network...")
	if err := client.ConnectAndLogin(ctx, server, details); err != nil {
		logger.Error("Failed to log in", log.Err(err))
		return
	}
	defer func() {
		_ = client.Disconnect()
	}()

	// 4. Invoke a Unified WebAPI Service via Protocol Router
	req := &pb.CPlayer_GetNickname_Request{
		Steamid: proto.Uint64(76561198000000000),
	}

	res, err := service.Unified[pb.CPlayer_GetNickname_Response](ctx, client.Service(), req)
	if err != nil {
		logger.Error("Service call failed", log.Err(err))
		return
	}

	logger.Info("Retrieved nickname via Service Router", log.String("nickname", res.GetNickname()))
}
```

## 🛠️ Key Orchestrator Design Patterns

### 1. Functional Options for Module Registration
Avoid registering modules imperatively post-initialization. Instead, supply your modules (e.g., TF2 engine, social plugins) during initialization using functional options:

```go
client, err := steam.NewClient(cfg,
    steam.WithLogger(logger),
    webtrading.WithModule(webtrading.DefaultConfig()),
)
```

### 2. Decoupled Event Subscriptions
Avoid blocking loops or modifying core code to intercept network lifecycle changes. Subscribe to events using G-man's central event bus:

```go
sub := client.Bus().Subscribe(&auth.LoggedOnEvent{})
go func() {
    for event := range sub.C() {
        if ev, ok := event.(*auth.LoggedOnEvent); ok {
            logger.Info("Established active session", log.Uint64("steam_id", ev.SteamID))
        }
    }
}()
```
