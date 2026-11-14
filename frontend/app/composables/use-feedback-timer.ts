import { onUnmounted } from 'vue'

/** Own a replaceable UI feedback timer and release it with its component. */
export function useFeedbackTimer() {
  let timeout: ReturnType<typeof setTimeout> | undefined
  let isDisposed = false

  const cancel = () => {
    if (timeout !== undefined) clearTimeout(timeout)
    timeout = undefined
  }

  onUnmounted(() => {
    isDisposed = true
    cancel()
  })

  return (callback: () => void, delayMilliseconds: number) => {
    if (isDisposed) return
    cancel()
    timeout = setTimeout(() => {
      timeout = undefined
      callback()
    }, delayMilliseconds)
  }
}
