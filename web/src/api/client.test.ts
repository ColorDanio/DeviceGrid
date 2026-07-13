import { describe, expect, it } from 'vitest'
import { getApiErrorMessage } from './client'

describe('getApiErrorMessage', () => {
  it('prefers a server-provided API message', () => {
    expect(getApiErrorMessage({ response: { data: { message: 'Permission denied' } } })).toBe('Permission denied')
  })

  it('falls back to the transport error message', () => {
    expect(getApiErrorMessage({ message: 'Network Error' })).toBe('Network Error')
  })
})
