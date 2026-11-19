import type { PostSeedAction } from './seed'

export interface GeneralSettings {
  launchOnStartup: boolean
  minimizeToTray: boolean
  closeToTray: boolean
  notificationsEnabled: boolean
  soundEnabled: boolean
  language: 'ru' | 'en'
  autoUpdateCheck: boolean
}

export interface SeedSettings {
  defaultTargetPlayers: number
  defaultMaxDurationMinutes: number
  defaultPostSeedAction: PostSeedAction
}

export interface PowerSettings {
  preventSleepDuringSeed: boolean
}

export interface PotatoSettings {
  enabled: boolean
  fpsLimit: number
  lowResolution: boolean
  lowCpuPriority: boolean
  autoRestoreOnExit: boolean
}

export interface GPUInfo {
  vendorId: number
  vendor: 'nvidia' | 'amd' | 'intel' | 'unknown'
  model: string
  dedicatedVram: number
  isRtx: boolean
}

export interface RestorePoint {
  id: string
  name: string
  description?: string
  createdAt: string
  files: any[]
  sha256: string
  sizeBytes: number
}

export interface SquadConfigStatus {
  errorMessage?: string
  configFound: boolean
  configPath: string
  hasBackup: boolean
  hasPendingRestore: boolean
  hasBaseline?: boolean
  backupPath: string
  isPotatoMode: boolean
  currentFpsCap: number
}

export interface AppSettings {
  general: GeneralSettings
  seed: SeedSettings
  power: PowerSettings
  potato: PotatoSettings
  webApiBaseUrl: string
}
