import type { ActiveSeedSession, SeedPlan } from '~/types'
import { bridgeAdapter } from './bridgeAdapter'
import { BridgeUnavailableError } from './errors'
import { validateSeedSession } from '~/utils/seed-validation'

export class WailsSeedAdapter {
  public async startSeed(plan: SeedPlan): Promise<ActiveSeedSession> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Seed engine operations require the Citadel Sentry desktop runtime.')
    }
    return validateSeedSession(await window.go!.main!.App!.StartSeed(plan))
  }

  public async stopSeed(): Promise<ActiveSeedSession> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Seed engine operations require the Citadel Sentry desktop runtime.')
    }
    return validateSeedSession(await window.go!.main!.App!.StopSeed())
  }

  public async getSession(): Promise<ActiveSeedSession> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Seed session status requires the Citadel Sentry desktop runtime.')
    }
    return validateSeedSession(await window.go!.main!.App!.GetSeedSession())
  }

  public async joinDirect(serverId: string): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Direct game join requires the Citadel Sentry desktop runtime.')
    }
    return window.go!.main!.App!.JoinServerDirect(serverId)
  }

  public async cancelPowerAction(): Promise<void> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Power action cancellation requires the Citadel Sentry desktop runtime.')
    }
    return window.go!.main!.App!.CancelPowerAction()
  }

  public async skipToNextServer(): Promise<boolean> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Seed engine operations require the Citadel Sentry desktop runtime.')
    }
    return window.go!.main!.App!.SkipToNextServer()
  }

  public async checkRunningSquadPotatoState(): Promise<{ isRunning: boolean; matchesPotato: boolean; currentFps: number }> {
    if (!bridgeAdapter.isAvailable()) {
      return { isRunning: false, matchesPotato: false, currentFps: 0 }
    }
    return window.go!.main!.App!.CheckRunningSquadPotatoState()
  }

  public async restartSquadWithPotato(plan: SeedPlan): Promise<ActiveSeedSession> {
    if (!bridgeAdapter.isAvailable()) {
      throw new BridgeUnavailableError('Seed engine operations require the Citadel Sentry desktop runtime.')
    }
    return validateSeedSession(await window.go!.main!.App!.RestartSquadWithPotato(plan))
  }
}

export const seedAdapter = new WailsSeedAdapter()
