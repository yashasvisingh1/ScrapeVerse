const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '')

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!response.ok) {
    let message = 'The service is unavailable right now.'
    try {
      const payload = await response.json()
      message = payload.error?.message ?? message
    } catch {
      // Keep the human-readable fallback when the response is not JSON.
    }
    throw new Error(message)
  }

  return response.json() as Promise<T>
}
