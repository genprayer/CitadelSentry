import type { AppSettings, SquadConfigStatus } from '~/types'

const MAX_DURATION_MINUTES = Math.floor((2 ** 63 - 1) / (60 * 1e9))

function requireObject(value: unknown, name: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${name} must be a JSON object`)
  }
  return value as Record<string, unknown>
}

function requireBooleans(value: Record<string, unknown>, name: string, keys: string[]) {
  for (const key of keys) {
    if (typeof value[key] !== 'boolean') throw new Error(`${name}.${key} must be an explicit boolean`)
  }
}

/** Accept an explicit native recovery status without coercing missing observations. */
export function validateSquadConfigStatus(value: unknown): SquadConfigStatus {
  const status = requireObject(value, 'Squad configuration status')
  if (status.errorMessage !== undefined) {
    if (typeof status.errorMessage !== 'string') throw new Error('Squad config errorMessage must be a string')
    if (status.errorMessage) throw new Error(status.errorMessage)
  }
  requireBooleans(status, 'Squad config status', ['configFound', 'hasBackup', 'hasPendingRestore', 'isPotatoMode'])
  for (const key of ['configPath', 'backupPath']) {
    if (typeof status[key] !== 'string') throw new Error(`${key} must be a string`)
  }
  if ((status.configFound && !status.configPath) || (status.hasBackup && !status.backupPath)) {
    throw new Error('Observed Squad files require their recorded paths')
  }
  if (typeof status.currentFpsCap !== 'number' || !Number.isFinite(status.currentFpsCap) || status.currentFpsCap < 0) {
    throw new Error('currentFpsCap must be a finite nonnegative number')
  }
  return value as SquadConfigStatus
}

/** Validate saved or native settings without inventing missing choices. */
export function validateAppSettings(value: unknown): AppSettings {
  const settings = requireObject(value, 'Application settings')
  const general = requireObject(settings.general, 'general')
  const seed = requireObject(settings.seed, 'seed')
  const power = requireObject(settings.power, 'power')
  const potato = requireObject(settings.potato, 'potato')
  if (general.autoUpdateCheck === undefined) {
    general.autoUpdateCheck = true
  }
  if (general.telemetryOptIn === undefined) {
    general.telemetryOptIn = false
  }
  if (general.telemetryPromptShown === undefined) {
    general.telemetryPromptShown = false
  }
  requireBooleans(general, 'general', ['launchOnStartup', 'minimizeToTray', 'closeToTray', 'notificationsEnabled', 'soundEnabled', 'autoUpdateCheck', 'telemetryOptIn', 'telemetryPromptShown'])
  requireBooleans(power, 'power', ['preventSleepDuringSeed'])
  requireBooleans(potato, 'potato', ['enabled', 'lowResolution', 'lowCpuPriority', 'autoRestoreOnExit'])
  if (general.language !== 'ru' && general.language !== 'en') throw new Error('Unsupported saved language')
  if (typeof seed.defaultTargetPlayers !== 'number' || !Number.isSafeInteger(seed.defaultTargetPlayers) || seed.defaultTargetPlayers <= 0) {
    throw new Error('defaultTargetPlayers must be a positive safe integer')
  }
  if (typeof seed.defaultMaxDurationMinutes !== 'number' || !Number.isSafeInteger(seed.defaultMaxDurationMinutes) ||
      seed.defaultMaxDurationMinutes <= 0 || seed.defaultMaxDurationMinutes > MAX_DURATION_MINUTES) {
    throw new Error('defaultMaxDurationMinutes must fit a positive duration')
  }
  if (typeof potato.fpsLimit !== 'number' || !Number.isSafeInteger(potato.fpsLimit) || potato.fpsLimit <= 0 || potato.fpsLimit > 60) {
    throw new Error('fpsLimit must be between 1 and 60')
  }
  if (typeof seed.defaultPostSeedAction !== 'string' || !['do_nothing', 'close_squad', 'sleep', 'shutdown'].includes(seed.defaultPostSeedAction)) {
    throw new Error('Unsupported defaultPostSeedAction')
  }
  if (typeof settings.webApiBaseUrl !== 'string' || settings.webApiBaseUrl !== settings.webApiBaseUrl.trim()) {
    throw new Error('webApiBaseUrl must be an absolute HTTP(S) base URL')
  }
  const endpoint = new URL(settings.webApiBaseUrl)
  if (!['https:', 'http:'].includes(endpoint.protocol) || !endpoint.hostname || settings.webApiBaseUrl.includes('?') || settings.webApiBaseUrl.includes('#')) {
    throw new Error('webApiBaseUrl must be an absolute HTTP(S) base URL without a query or fragment')
  }
  return value as AppSettings
}
