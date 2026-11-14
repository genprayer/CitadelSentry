# Citadel Seeder Frontend

Tactical desktop auto-seeder frontend built with **Nuxt 4 / Vue 3 / TypeScript / Tailwind CSS / Pinia**, designed to be embedded into the **Wails (Go)** desktop runtime.

## Architecture

This frontend adheres strictly to Clean Architecture and decoupled frontend principles:

```text
┌────────────────────────────────────────────────────────┐
│                      Pages & Layouts                   │
│   (index.vue, seed/index.vue, servers, settings, login)│
├────────────────────────────────────────────────────────┤
│                     Feature Components                 │
│      (ActiveSeedCard, ServerList, SteamLinkWarning)    │
├────────────────────────────────────────────────────────┤
│                 Composables & Pinia Stores             │
│       (useAuth, useSeed, useServers, useBridge)        │
├────────────────────────────────────────────────────────┤
│                    Application Services                │
│    (AuthService, ServerService, SeedService, Settings) │
├────────────────────────────────────────────────────────┤
│                     Wails Adapters                     │
│  (WailsBridgeAdapter, AuthAdapter, SeedAdapter, etc.)  │
├────────────────────────────────────────────────────────┤
│              Wails IPC Bridge / Runtime                │
│          (window.go.main.App, window.runtime)          │
└────────────────────────────────────────────────────────┘
```

### Key Highlights
1. **Decoupled Bridge Interface**: Vue components never directly touch OS/system APIs or `window.go`. All communication passes through typed adapters (`app/adapters/wails/`) and domain services (`app/services/`).
2. **Honest Operational States**: No fake mock loops, simulated seed timers, or dummy online counts. If the bridge is absent or disconnected, the UI cleanly informs the operator (`Browser Standby`, `Offline Bridge`).
3. **Mandatory Steam Validation**: In compliance with Citadel authentication rules, users must have a linked Steam account (`ExternalAccount(provider="steam")`) before launching seed missions. A prominent warning banner guides the user to link Steam via Citadel Web settings if unlinked.
4. **Desktop Native Shell**: Built with custom tactical titlebar supporting Wails window dragging (`--wails-draggable: drag`), minimize, maximize, and exit controls.
5. **Startup & Diagnostics**: Sleek startup splash sequence showing bridge readiness, session status, and host platform diagnostics.

---

## Directory Structure

```text
frontend/
├── app/
│   ├── adapters/
│   │   └── wails/            # Wails Go IPC bridge wrappers & browser fallbacks
│   ├── assets/
│   │   └── css/main.css      # Tailwind & tactical desktop styles
│   ├── components/
│   │   ├── app/              # Titlebar, sidebar, header, status bar, splash, banner
│   │   ├── auth/             # Login card, Steam link warning
│   │   ├── seed/             # ActiveSeedCard HUD, SeedConfigModal, Queue, StateIndicator
│   │   ├── server/           # ServerCard, ServerList, ServerDetailsModal
│   │   ├── settings/         # General, Seed, and Power settings
│   │   └── ui/               # BaseButton, BaseCard, BaseInput, BaseModal, BaseSelect, etc.
│   ├── composables/          # Presentation composables (useAuth, useSeed, etc.)
│   ├── constants/            # Event names, presets, storage keys
│   ├── layouts/              # default.vue, auth.vue
│   ├── pages/                # index.vue, seed/index.vue, servers/index.vue, settings.vue, login.vue
│   ├── services/             # Application domain logic & plan validators
│   ├── stores/               # Pinia stores (auth, seed, servers, settings, bridge)
│   ├── types/                # Domain TypeScript contracts
│   ├── utils/                # Time, formatters, calculations
│   └── app.vue               # Root application entry with startup synchronization
├── eslint.config.js          # ESLint flat configuration
├── nuxt.config.ts            # Nuxt 4 configuration (ssr: false, static generate, telemetry: false)
├── package.json
├── tailwind.config.ts        # Dark gaming / tactical Citadel theme
└── tsconfig.json
```

---

## Wails Go Contracts

The frontend expects the following Go methods exposed on `App`:

### 1. Bridge & System
* `GetBridgeStatus() -> WailsBridgeStatus`
* `OpenExternalUrl(url string) -> error`
* `MinimizeWindow() -> error`
* `MaximizeWindow() -> error`
* `CloseWindow() -> error`

### 2. Authentication (Citadel Web OAuth)
* `StartCitadelOAuth() -> { authUrl string, pollKey string }`
* `ExchangeToken(pollKey string) -> AuthResult`
* `GetCachedAuth() -> AuthResult | null`
* `Logout() -> error`

### 3. Server Fleet
* `FetchServers() -> []Server`
* `GetServerDetails(serverId string) -> Server`

### 4. Seed Engine
* `StartSeed(plan SeedPlan) -> ActiveSeedSession`
* `StopSeed() -> ActiveSeedSession`
* `GetSeedSession() -> ActiveSeedSession`
* `JoinServerDirect(serverId string) -> bool`

### 5. Settings
* `GetSettings() -> AppSettings`
* `SaveSettings(settings AppSettings) -> bool`

### 6. Wails Runtime Events Emitted to Frontend
* `citadel:seed:state_changed` (`ActiveSeedSession`)
* `citadel:seed:progress` (`{ currentPlayerCount: number, elapsedSeconds: number, remainingSeconds?: number, queuePosition?: number }`)
* `citadel:servers:updated` (`[]Server`)

---

## Development & Build Commands

### Install Dependencies
```bash
npm install
```

### Linting
```bash
npm run lint
```

### Type Checking
```bash
npm run typecheck
```

### Development Server (Browser Standby)
```bash
npm run dev
```

### Production Static Generate (for Wails Embedding)
```bash
npm run generate
```
Build output is saved to `.output/public/` for embedding into the Go executable via `//go:embed all:frontend/.output/public`.
