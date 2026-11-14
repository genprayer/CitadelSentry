export function formatSecondsToHms(totalSeconds: number): string {
  if (totalSeconds < 0) totalSeconds = 0
  const hrs = Math.floor(totalSeconds / 3600)
  const mins = Math.floor((totalSeconds % 3600) / 60)
  const secs = Math.floor(totalSeconds % 60)

  if (hrs > 0) {
    return `${hrs}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
  }
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}

export function formatDurationMinutes(minutes: number): string {
  if (minutes < 60) {
    return `${minutes}m`
  }
  const hrs = Math.floor(minutes / 60)
  const remMins = minutes % 60
  if (remMins === 0) {
    return `${hrs}h`
  }
  return `${hrs}h ${remMins}m`
}
