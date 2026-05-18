<div align="center">

# G-MAN Framework Examples

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

This directory contains production-ready examples of bots built with the G-MAN framework. These examples demonstrate how to use the module system, the trading engine, and custom business logic.

## 📁 Examples Overview

| Example | Description | Key Features |
| :--- | :--- | :--- |
| [**Trading Bot**](trading_bot) | A generic Steam trading bot template. | JSON storage, automated 2FA, simple donation acceptance. |
| [**TF2 Bot**](tf2_bot) | A professional Team Fortress 2 trading bot. | PriceDB integration, stock limits, dupe checks, automatic change settlement. |
| [**Custom Logic Bot**](custom_logic_bot) | Demonstrates how to extend the bot with custom logic. | Friend whitelisting, business hours maintenance mode, middleware chaining. |

---

## 🚀 Getting Started

To run any of the examples, follow these steps:

1. **Set Environment Variables:**
   The bots require your Steam credentials to be set in your environment.
   ```bash
   # Windows (PowerShell)
   $env:STEAM_USER="your_username"
   $env:STEAM_PASS="your_password"

   # Linux/macOS
   export STEAM_USER="your_username"
   export STEAM_PASS="your_password"
   ```

2. **Run the example:**
   ```bash
   go run ./examples/trading_bot
   ```

---

## 🏗 Common Patterns

### 1. Middleware Chain
G-man uses an "Onion" middleware pattern for trade logic. Each middleware can either:
- Reach a **Verdict** (Accept/Decline/Counter) and stop the chain.
- Pass the context to the **Next** middleware in the chain.

### 2. Automated Processor
Instead of manually accepting trades, use `SetOfferHandler` with the built-in `engine.NewBotHandler`. This handles:
- **Item Locking:** Prevents selling the same item twice.
- **Retries:** Automatically retries WebAPI calls with exponential backoff.
- **Concurrency:** Processes offers sequentially to avoid inventory race conditions.

### 3. Persistent Storage
All examples use `storage.json` to persist:
- Login Tokens (avoids frequent 2FA prompts).
- Steam Guard secrets.
- Machine-specific hardware IDs.

---

## 🛠 Advanced TF2 Features (TF2 Bot Example)

The TF2 bot is the most advanced example and requires additional setup:
- **`trading_config.json`**: Generated on the first run. Allows you to set per-SKU stock limits and prices.
- **API Keys**: Requires `BPTF_API_KEY` and `BPTF_USER_TOKEN` for dupe checks and reputation monitoring.
- **PriceDB**: Uses a unified price feed to ensure your bot always buys low and sells high.
