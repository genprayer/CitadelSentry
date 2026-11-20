import type { Server } from './server'
import type { ActiveSeedSession, SeedPlan } from './seed'
import type { AppSettings } from './settings'

export type PlatformType = 'windows' | 'linux' | 'darwin' | 'browser' | 'unknown'

export interface WailsBridgeStatus {
  isAvailable: boolean
  platform: PlatformType
  appVersion: string
  squadRunning: boolean | null
  steamRunning: boolean | null
  processStatusError?: string
  canSetLowCpuPriority?: boolean
  lastPingAt: string | null
  logFilePath?: string
  squadConfigStatus?: import('./settings').SquadConfigStatus
}

export interface UpdateInfo {
  currentVersion: string
  latestVersion: string
  available: boolean
  assetName: string
  assetSize: number
  releaseUrl: string
}

export const BRIDGE_EVENTS = {
  SEED_STATE_CHANGED: 'citadel:seed:state_changed',
  SEED_PROGRESS: 'citadel:seed:progress',
  SERVER_LIST_UPDATED: 'citadel:servers:updated',
  SQUAD_PROCESS_STATUS: 'citadel:squad:status',
  SQUAD_CONFIG_STATUS: 'squad:config_status',
  POWER_ACTION_IMPENDING: 'citadel:power_action_impending',
  POWER_ACTION_CANCELLED: 'citadel:power_action_cancelled',
  APP_LOG: 'citadel:log'
} as const

export interface WailsGoAppApi {
  // Bridge / System
  GetBridgeStatus(): Promise<WailsBridgeStatus>
  GetSystemStatus(): Promise<WailsBridgeStatus>
  GetLogFilePath(): Promise<string>
  GetRecentLogs(limit: number): Promise<string[]>
  ClearLogs(): Promise<void>
  OpenLogsFolder(): Promise<string>
  OpenExternalUrl(url: string): Promise<void>
  OpenExternal(url: string): Promise<void>
  MinimizeWindow(): Promise<void>
  MaximizeWindow(): Promise<void>
  CloseWindow(): Promise<void>
  CancelPowerAction(): Promise<void>
  CheckForUpdates(): Promise<UpdateInfo>
  GetUpdateLastError(): Promise<string>
  InstallUpdate(version: string): Promise<void>

  // Servers
  FetchServers(): Promise<Server[]>
  GetServers(): Promise<Server[]>
  GetServerDetails(serverId: string): Promise<Server>

  // Seed Engine
  StartSeed(plan: SeedPlan): Promise<ActiveSeedSession>
  StopSeed(): Promise<ActiveSeedSession>
  GetSeedSession(): Promise<ActiveSeedSession>
  JoinServerDirect(serverId: string): Promise<boolean>
  SkipToNextServer(): Promise<boolean>
  CheckRunningSquadPotatoState(): Promise<{ isRunning: boolean; matchesPotato: boolean; currentFps: number }>
  RestartSquadWithPotato(plan: SeedPlan): Promise<ActiveSeedSession>

  // Settings
  GetSettings(): Promise<AppSettings>
  SaveSettings(settings: AppSettings): Promise<boolean>
  SetTelemetryOptIn(optIn: boolean): Promise<void>
  GetTelemetryPreview(): Promise<any>
  GetPotatoSettings(): Promise<import('./settings').PotatoSettings>
  SavePotatoSettings(settings: import('./settings').PotatoSettings): Promise<boolean>
  RestoreSquadConfig(): Promise<boolean>
  SaveCurrentSquadConfigAsBackup(): Promise<boolean>
  GetSquadConfigStatus(): Promise<import('./settings').SquadConfigStatus>
  GetGPUInfo(): Promise<import('./settings').GPUInfo>
  RestoreInitialBaseline(): Promise<boolean>
  HasInitialBaseline(): Promise<boolean>
  CreateRestorePoint(name: string, description: string): Promise<import('./settings').RestorePoint>
  ListRestorePoints(): Promise<import('./settings').RestorePoint[]>
  RestoreRestorePoint(id: string): Promise<boolean>
  DeleteRestorePoint(id: string): Promise<boolean>
}

declare global {
  interface Window {
    runtime?: {
      EventsOn: (eventName: string, callback: (...args: any[]) => void) => () => void
      EventsOnce: (eventName: string, callback: (...args: any[]) => void) => void
      EventsEmit: (eventName: string, ...args: any[]) => void
      WindowMinimise: () => void
      WindowMaximise: () => void
      WindowUnmaximise: () => void
      WindowToggleMaximise: () => void
      WindowHide: () => void
      WindowShow: () => void
      WindowClose: () => void
      Quit: () => void
      BrowserOpenURL: (url: string) => void
    }
    go?: {
      main?: {
        App?: WailsGoAppApi
      }
    }
  }
}
