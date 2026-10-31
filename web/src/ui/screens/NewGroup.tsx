import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { User } from '../../api/client'
import { ledgers } from '../../session'
import { Figure } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { navigate } from '../router'
import { problemText, useOperation } from './operation'
import { checkPerson, nobody, pendingMember, type Person, PersonFields, type PersonErrors } from './PersonFields'
import { useUsedCurrencies } from '../forms/data'
import { CurrencySelect } from './Welcome'

// The API takes at most this many pending Members with a new Group.
const MOST_PENDING = 50

export function NewGroup({ user }: { user: User }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string>()
  const [currency, setCurrency] = useState(user.preferredCurrency)
  const used = useUsedCurrencies(user.id)
  const [people, setPeople] = useState<Person[]>([])
  const [draft, setDraft] = useState(nobody)
  const [draftErrors, setDraftErrors] = useState<PersonErrors>()
  const { busy, error, run } = useOperation()

  const typed = draft.name.trim() !== '' || draft.email.trim() !== ''
  const full = people.length + (typed ? 1 : 0) > MOST_PENDING

  const addDraft = () => {
    const errors = checkPerson(t, draft)
    setDraftErrors(errors)
    if (errors) return
    setPeople([...people, draft])
    setDraft(nobody)
  }

  const save = () => {
    const trimmed = name.trim()
    setNameError(trimmed ? undefined : t('newGroup.nameRequired'))
    // Someone typed but not added yet goes along, rather than getting lost.
    const errors = typed ? checkPerson(t, draft) : undefined
    setDraftErrors(errors)
    if (!trimmed || errors || full) return
    const pendingMembers = (typed ? [...people, draft] : people).map(pendingMember)
    void run(
      () => ledgers.createGroup({ name: trimmed, currency, simplifyDebts: true, pendingMembers }),
      (group) => {
        toast(t('newGroup.created'))
        navigate({ name: 'ledger', ledgerId: group.id }, { replace: true })
      },
    )
  }

  return (
    <Screen
      back={{ name: 'home' }}
      title={t('newGroup.title')}
      dock={
        <div className="dock-stack">
          {!online && <p className="dock-note">{t('common.onlineOnly')}</p>}
          <button type="button" className="button button-primary button-block" disabled={busy || !online || full} title={online ? undefined : t('common.onlineOnly')} onClick={save}>
            {busy ? t('newGroup.saving') : t('newGroup.save')}
          </button>
        </div>
      }
    >
      <form
        className="form"
        noValidate
        onSubmit={(e) => {
          e.preventDefault()
          save()
        }}
      >
        <div className="field">
          <label htmlFor={`${ids}-name`}>{t('newGroup.name')}</label>
          <input
            id={`${ids}-name`}
            className="input"
            autoComplete="off"
            maxLength={80}
            placeholder={t('newGroup.namePlaceholder')}
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              if (e.target.value.trim()) setNameError(undefined)
            }}
            aria-invalid={!!nameError}
            aria-describedby={nameError ? `${ids}-name-error` : undefined}
          />
          {nameError && (
            <p className="field-error" id={`${ids}-name-error`} role="alert">
              {nameError}
            </p>
          )}
        </div>
        <div className="field">
          <label htmlFor={`${ids}-currency`}>{t('newGroup.currency')}</label>
          <CurrencySelect id={`${ids}-currency`} value={currency} onChange={setCurrency} describedBy={`${ids}-currency-hint`} used={used} />
          <p className="hint" id={`${ids}-currency-hint`}>
            {t('newGroup.currencyHint')}
          </p>
        </div>
      </form>

      <section className="section" aria-labelledby={`${ids}-pending`}>
        <h2 id={`${ids}-pending`} className="section-title">
          {t('newGroup.pendingTitle')}
        </h2>
        <p className="hint">{t('newGroup.pendingHint')}</p>
        {people.length > 0 && (
          <ul className="rows">
            {people.map((person, index) => (
              <li key={index} className="row">
                <span className="row-glyph">
                  <Figure state="pending" />
                </span>
                <span className="row-main">
                  <span className="row-title">{person.name.trim()}</span>
                  {person.email.trim() && <span className="row-sub">{person.email.trim()}</span>}
                </span>
                <span className="row-end">
                  <button
                    type="button"
                    className="button button-quiet button-small"
                    aria-label={t('newGroup.removeLabel', { name: person.name.trim() })}
                    onClick={() => setPeople(people.filter((_, i) => i !== index))}
                  >
                    {t('newGroup.remove')}
                  </button>
                </span>
              </li>
            ))}
          </ul>
        )}
        <form
          className="form"
          noValidate
          onSubmit={(e) => {
            e.preventDefault()
            addDraft()
          }}
        >
          <PersonFields
            value={draft}
            onChange={(person) => {
              setDraft(person)
              setDraftErrors(undefined)
            }}
            errors={draftErrors}
          />
          <div className="button-row">
            <button type="submit" className="button button-secondary" disabled={people.length >= MOST_PENDING}>
              {t('newGroup.addPerson')}
            </button>
          </div>
          {full && <p className="hint">{t('newGroup.tooMany', { count: MOST_PENDING })}</p>}
        </form>
      </section>

      {error && (
        <p className="notice notice-problem" role="alert">
          {problemText(t, error)}
        </p>
      )}
    </Screen>
  )
}
