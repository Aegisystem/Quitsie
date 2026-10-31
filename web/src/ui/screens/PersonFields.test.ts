import { describe, expect, it } from 'vitest'
import { checkPerson, pendingMember } from './PersonFields'

const t = ((key: string) => key) as Parameters<typeof checkPerson>[0]

describe('a pending person', () => {
  it('needs a name', () => {
    expect(checkPerson(t, { name: '   ', email: '' })).toEqual({ name: 'person.nameRequired' })
  })

  it('may have no email, but not a broken one', () => {
    expect(checkPerson(t, { name: 'Beto', email: '' })).toBeUndefined()
    expect(checkPerson(t, { name: 'Beto', email: ' beto@example.com ' })).toBeUndefined()
    expect(checkPerson(t, { name: 'Beto', email: 'beto' })).toEqual({ email: 'person.emailInvalid' })
  })

  it('leaves the email out when there is none, which is how the API removes one', () => {
    expect(pendingMember({ name: ' Beto ', email: '  ' })).toEqual({ name: 'Beto' })
    expect(pendingMember({ name: 'Beto', email: ' beto@example.com ' })).toEqual({ name: 'Beto', email: 'beto@example.com' })
  })
})
