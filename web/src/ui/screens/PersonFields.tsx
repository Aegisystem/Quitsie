import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { NewPendingMember } from '../../session'

// The name and optional email of a pending Member or a pending Friend.

export interface Person {
  name: string
  email: string
}

export const nobody: Person = { name: '', email: '' }

export interface PersonErrors {
  name?: string
  email?: string
}

type T = ReturnType<typeof useTranslation>['t']

export function checkPerson(t: T, { name, email }: Person): PersonErrors | undefined {
  const errors: PersonErrors = {}
  if (!name.trim()) errors.name = t('person.nameRequired')
  if (email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) errors.email = t('person.emailInvalid')
  return errors.name || errors.email ? errors : undefined
}

// Without an email it is left out, which is also how the API removes one.
export function pendingMember({ name, email }: Person): NewPendingMember {
  return { name: name.trim(), ...(email.trim() && { email: email.trim() }) }
}

// `emailHint` replaces the usual one, for example when editing a Member whose
// email, if any, only who gave it can see.
export function PersonFields({
  value,
  onChange,
  errors,
  emailHint,
}: {
  value: Person
  onChange: (person: Person) => void
  errors?: PersonErrors
  emailHint?: string
}) {
  const { t } = useTranslation()
  const ids = useId()
  return (
    <>
      <div className="field">
        <label htmlFor={`${ids}-name`}>{t('person.name')}</label>
        <input
          id={`${ids}-name`}
          className="input"
          autoCapitalize="words"
          autoComplete="off"
          maxLength={80}
          value={value.name}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
          aria-invalid={!!errors?.name}
          aria-describedby={errors?.name ? `${ids}-name-error` : undefined}
        />
        {errors?.name && (
          <p className="field-error" id={`${ids}-name-error`} role="alert">
            {errors.name}
          </p>
        )}
      </div>
      <div className="field">
        <label htmlFor={`${ids}-email`}>{t('person.email')}</label>
        <input
          id={`${ids}-email`}
          className="input"
          type="email"
          inputMode="email"
          autoCapitalize="none"
          autoComplete="off"
          maxLength={254}
          value={value.email}
          onChange={(e) => onChange({ ...value, email: e.target.value })}
          aria-invalid={!!errors?.email}
          aria-describedby={errors?.email ? `${ids}-email-error` : `${ids}-email-hint`}
        />
        {errors?.email ? (
          <p className="field-error" id={`${ids}-email-error`} role="alert">
            {errors.email}
          </p>
        ) : (
          <p className="hint" id={`${ids}-email-hint`}>
            {emailHint ?? t('person.emailHint')}
          </p>
        )}
      </div>
    </>
  )
}
