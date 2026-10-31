import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { User } from '../../api/client'
import { liveHome } from '../../local'
import { ledgers, replica, useLive } from '../../session'
import { Figure } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { navigate } from '../router'
import { problemText, useOperation } from './operation'
import { checkPerson, nobody, pendingMember, PersonFields, type PersonErrors } from './PersonFields'
import { usersToBefriend } from './usersToBefriend'

// A Friend is someone from a shared Group or, for anyone else, a pending Friend
// who Claims their place with an Invite link.
export function NewFriend({ user }: { user: User }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [home] = useLive('home', () => liveHome(replica))
  const known = useMemo(() => usersToBefriend(home ?? [], user.id), [home, user.id])
  const [friend, setFriend] = useState(nobody)
  const [errors, setErrors] = useState<PersonErrors>()
  const { busy, error, run } = useOperation()
  const offlineTitle = online ? undefined : t('common.onlineOnly')

  const addPending = () => {
    const found = checkPerson(t, friend)
    setErrors(found)
    if (found) return
    void run(
      () => ledgers.createFriendship(pendingMember(friend)),
      ({ id }) => {
        toast(t('newFriend.added', { name: friend.name.trim() }))
        navigate({ name: 'inviteLink', ledgerId: id }, { replace: true })
      },
    )
  }

  const addUser = (userId: string, name: string) =>
    void run(
      () => ledgers.createFriendship({ userId }),
      ({ id }) => {
        toast(t('newFriend.added', { name }))
        navigate({ name: 'ledger', ledgerId: id }, { replace: true })
      },
    )

  return (
    <Screen
      back={{ name: 'home' }}
      title={t('newFriend.title')}
      dock={
        <div className="dock-stack">
          {!online && <p className="dock-note">{t('common.onlineOnly')}</p>}
          <button type="button" className="button button-primary button-block" disabled={busy || !online} title={offlineTitle} onClick={addPending}>
            {busy ? t('newFriend.saving') : t('newFriend.save')}
          </button>
        </div>
      }
    >
      {known.length > 0 && (
        <section className="section" aria-labelledby={`${ids}-known`}>
          <h2 id={`${ids}-known`} className="section-title">
            {t('newFriend.knownTitle')}
          </h2>
          <p className="hint">{t('newFriend.knownHint')}</p>
          <ul className="rows">
            {known.map(({ userId, name, groups }) => (
              <li key={userId} className="row">
                <span className="row-glyph">
                  <Figure />
                </span>
                <span className="row-main">
                  <span className="row-title">{name}</span>
                  <span className="row-sub">{groups.join(', ')}</span>
                </span>
                <span className="row-end">
                  <button
                    type="button"
                    className="button button-secondary button-small"
                    disabled={busy || !online}
                    title={offlineTitle}
                    aria-label={t('newFriend.addLabel', { name })}
                    onClick={() => addUser(userId, name)}
                  >
                    {t('newFriend.add')}
                  </button>
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className="section" aria-labelledby={`${ids}-pending`}>
        <h2 id={`${ids}-pending`} className="section-title">
          {t('newFriend.pendingTitle')}
        </h2>
        <p className="hint">{t('newFriend.pendingHint')}</p>
        <form
          className="form"
          noValidate
          onSubmit={(e) => {
            e.preventDefault()
            addPending()
          }}
        >
          <PersonFields
            value={friend}
            onChange={(person) => {
              setFriend(person)
              setErrors(undefined)
            }}
            errors={errors}
          />
        </form>
      </section>

      {error && (
        <p className="notice notice-problem" role="alert">
          {problemText(t, error, { friendship_exists: t('newFriend.exists'), not_found: t('newFriend.notShared') })}
        </p>
      )}
    </Screen>
  )
}
