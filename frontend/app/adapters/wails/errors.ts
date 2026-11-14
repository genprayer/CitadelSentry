export class BridgeUnavailableError extends Error {
  constructor(message = 'Wails desktop bridge is unavailable. Running in browser or disconnected.') {
    super(message)
    this.name = 'BridgeUnavailableError'
  }
}

export class AuthOperationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'AuthOperationError'
  }
}

export class SeedOperationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'SeedOperationError'
  }
}
