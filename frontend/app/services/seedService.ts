import { seedAdapter } from '~/adapters/wails/seedAdapter'
import { SeedOperationError } from '~/adapters/wails/errors'
import type { ActiveSeedSession, SeedPlan, Server } from '~/types'
import { validateSeedPlan } from '~/utils/seed-validation'

export class SeedService {
  public static readonly TARGET_PRESETS = [35, 45, 55, 65, 75]
  public static readonly DURATION_PRESETS = [30, 45, 60, 90, 120]

  public validatePlan(plan: SeedPlan, server?: Server | null): { valid: boolean; error?: string } {
    try {
      validateSeedPlan(plan)
    } catch (error) {
      return { valid: false, error: String(error) }
    }
    if (server) {
      if (!Number.isSafeInteger(server.maxPlayers) || server.maxPlayers <= 0) return { valid: false, error: 'Server capacity is unknown.' }
      if (plan.targetPlayerCount > server.maxPlayers) return { valid: false, error: `Target player count cannot exceed ${server.maxPlayers}.` }
    }
    // The backend resolves the whole current fleet and skips already met targets.
    return { valid: true }
  }

  public async startSeed(plan: SeedPlan, server?: Server | null): Promise<ActiveSeedSession> {
    const validation = this.validatePlan(plan, server)
    if (!validation.valid) {
      throw new SeedOperationError(validation.error || 'Invalid seed configuration')
    }
    return seedAdapter.startSeed(plan)
  }

  public async stopSeed(): Promise<ActiveSeedSession> {
    return seedAdapter.stopSeed()
  }

  public async getSession(): Promise<ActiveSeedSession> {
    return seedAdapter.getSession()
  }

  public async joinServerDirect(serverId: string): Promise<boolean> {
    return seedAdapter.joinDirect(serverId)
  }

  public async cancelPowerAction(): Promise<void> {
    return seedAdapter.cancelPowerAction()
  }

  public async skipToNextServer(): Promise<boolean> {
    return seedAdapter.skipToNextServer()
  }

  public async checkRunningSquadPotatoState(): Promise<{ isRunning: boolean; matchesPotato: boolean; currentFps: number }> {
    return seedAdapter.checkRunningSquadPotatoState()
  }

  public async restartSquadWithPotato(plan: SeedPlan, server?: Server | null): Promise<ActiveSeedSession> {
    const validation = this.validatePlan(plan, server)
    if (!validation.valid) {
      throw new SeedOperationError(validation.error || 'Invalid seed configuration')
    }
    return seedAdapter.restartSquadWithPotato(plan)
  }
}

export const seedService = new SeedService()
