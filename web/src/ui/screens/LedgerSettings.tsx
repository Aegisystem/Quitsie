import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Group, Member, User } from '../../api/client'
import type { Balance } from '../../domain'
import { type LedgerSummary, liveExpenses, liveLedger, liveSettlements } from '../../local'
import { ledgers, type LossyRecords, replica } from '../../session'
import { useLive } from '../data'
import { Amount, toneOf } from '../components/Balance'
import { Figure, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { href, navigate } from '../router'
import { problemText, useOperation } from './operation'
import { checkPerson, nobody, pendingMember, type Person, PersonFields, type PersonErrors } from './PersonFields'
import { useUsedCurrencies } from '../forms/data'
import { CurrencySelect } from './Welcome'

// Lines the actions up with the name, under the figure of the row.
const underTheName = { marginLeft: 44, paddingBottom: 14 }

export function LedgerSettings({ ledgerId, user }: { ledgerId: string; user: User }) {
  const { t } = useTranslation()
  const [summary, loaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))

  if (!summary) {
    return (
      <Screen back={{ name: 'home' }} title={t('ledgerSettings.title')}>
        {loaded && <p className="empty">{t('ledger.unknown')}</p>}
      </Screen>
    )
  }
  return (
    <Screen back={{ name: 'ledger', ledgerId }} title={t('ledgerSettings.title')}>
      {summary.ledger.kind === 'group' ? (
        <GroupSettings group={summary.ledger} summary={summary} user={user} />
      ) : (
        <FriendshipSettings summary={summary} user={user} />
      )}
    </Screen>
  )
}

// A Friendship has no name or Group currency, and the API does not edit one yet.
function FriendshipSettings({ summary, user }: { summary: LedgerSummary; user: User }) {
  const { t } = useTranslation()
  const friend = summary.members.find((member) => member.userId !== user.id)
  return (
    <>
      <h2 className="page-title">{friend?.name}</h2>
      <p className="lead">{t('ledger.friendship')}</p>
      {friend && !friend.userId && (
        <div className="notice">
          <p>{t('ledgerSettings.friendPending', { name: friend.name })}</p>
          <a className="button button-secondary" href={href({ name: 'inviteLink', ledgerId: summary.ledger.id })}>
            {t('ledgerSettings.showLink')}
          </a>
        </div>
      )}
    </>
  )
}

function GroupSettings({ group, summary, user }: { group: Group; summary: LedgerSummary; user: User }) {
  const { t } = useTranslation()
  // The User comes first.
  const active = summary.members.filter((member) => member.status === 'active').sort((a, b) => Number(b.userId === user.id) - Number(a.userId === user.id))
  const balancesOf = (memberId: string) => summary.balances.filter((balance) => balance.participantId === memberId)
  const me = active.find((member) => member.userId === user.id)
  const removed = summary.members.filter((member) => member.status === 'former' && member.removed)

  return (
    <>
      <GroupForm group={group} user={user} />
      <section className="section" aria-labelledby="members-title">
        <h2 id="members-title" className="section-title">
          {t('ledgerSettings.membersTitle')}
        </h2>
        <ul className="rows">
          {active.map((member) => (
            <MemberRow key={member.id} groupId={group.id} member={member} mine={member.id === me?.id} balances={balancesOf(member.id)} />
          ))}
        </ul>
        <AddPendingMember groupId={group.id} />
      </section>
      {removed.length > 0 && (
        <section className="section" aria-labelledby="removed-title">
          <h2 id="removed-title" className="section-title">
            {t('ledgerSettings.removedTitle')}
          </h2>
          <p className="hint">{t('ledgerSettings.removedHint')}</p>
          <ul className="rows">
            {removed.map((member) => (
              <RemovedRow key={member.id} groupId={group.id} member={member} />
            ))}
          </ul>
        </section>
      )}
      {me && <LeaveOrDelete group={group} me={me} members={active} myBalances={balancesOf(me.id)} settled={summary.balances.length === 0} />}
    </>
  )
}

// Name and Group currency. What the User has not touched follows the replica,
// so a change from another Member shows, and the form is clean once saved.
function GroupForm({ group, user }: { group: Group; user: User }) {
  const used = useUsedCurrencies(user.id)
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [typedName, setTypedName] = useState<string>()
  const [pickedCurrency, setPickedCurrency] = useState<string>()
  const [nameError, setNameError] = useState<string>()
  const [lossy, setLossy] = useState<{ currency: string; records: LossyRecords }>()
  const { busy, error, run, reset } = useOperation()

  const name = typedName ?? group.name
  const currency = pickedCurrency ?? group.currency
  const renamed = name.trim() !== group.name
  const relabels = currency !== group.currency

  const clean = () => {
    setTypedName(undefined)
    setPickedCurrency(undefined)
    setNameError(undefined)
    setLossy(undefined)
    reset()
  }

  const save = () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError(t('ledgerSettings.nameRequired'))
      return
    }
    setLossy(undefined)
    void run(
      () => ledgers.updateGroup(group.id, { ...(renamed && { name: trimmed }), ...(relabels && { currency }) }),
      () => {
        clean()
        toast(relabels ? t('ledgerSettings.relabeled', { currency }) : t('ledgerSettings.saved'))
      },
      (refused) => {
        if (refused.kind === 'problem' && refused.lossy) setLossy({ currency, records: refused.lossy })
      },
    )
  }

  return (
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
          value={name}
          onChange={(e) => {
            setTypedName(e.target.value)
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
        <CurrencySelect
          id={`${ids}-currency`}
          value={currency}
          onChange={(next) => {
            setPickedCurrency(next)
            setLossy(undefined)
          }}
          describedBy={`${ids}-currency-hint`}
          used={used}
        />
        <p className="hint" id={`${ids}-currency-hint`}>
          {t('ledgerSettings.currencyHint')}
        </p>
      </div>
      {relabels && (
        <div className="notice">
          <p>{t('ledgerSettings.relabelExample', { from: group.currency, to: currency })}</p>
          <p>{t('ledgerSettings.relabelRecords')}</p>
        </div>
      )}
      {lossy ? (
        <LossyAmounts groupId={group.id} currency={lossy.currency} records={lossy.records} />
      ) : (
        error && (
          <p className="notice notice-problem" role="alert">
            {problemText(t, error, { invalid_amounts: t('ledgerSettings.relabelBalance', { currency }) })}
          </p>
        )
      )}
      {(renamed || relabels) && (
        <div className="button-row">
          <button type="submit" className="button button-primary" disabled={busy || !online} title={online ? undefined : t('common.onlineOnly')}>
            {busy ? t('account.saving') : t('account.save')}
          </button>
          <button type="button" className="button button-quiet" onClick={clean}>
            {t('common.cancel')}
          </button>
        </div>
      )}
      {(renamed || relabels) && !online && <p className="hint">{t('common.onlineOnly')}</p>}
    </form>
  )
}

// The server refused the change because these amounts would lose decimals. Deleted
// records count, as they can be restored (docs/reglas-de-negocio.md).
function LossyAmounts({ groupId, currency, records }: { groupId: string; currency: string; records: LossyRecords }) {
  const { t } = useTranslation()
  const [expenses = []] = useLive(`expenses:${groupId}`, () => liveExpenses(replica, groupId))
  const [settlements = []] = useLive(`settlements:${groupId}`, () => liveSettlements(replica, groupId))
  const lossyExpenses = expenses.filter(({ id }) => records.expenseIds.includes(id))
  const lossySettlements = settlements.filter(({ id }) => records.settlementIds.includes(id))
  const count = records.expenseIds.length + records.settlementIds.length

  return (
    <div className="notice" role="alert">
      <p className="notice-problem">{t('ledgerSettings.lossyTitle', { count, currency })}</p>
      <ul className="rows">
        {lossyExpenses.map(({ id, description, total, currency: from, deletedAt }) => (
          <li key={id} className="row" style={{ gridTemplateColumns: 'minmax(0, 1fr) auto', minHeight: 0 }}>
            <span className="row-main">
              <span className="row-title">{description}</span>
              {deletedAt && <span className="row-sub">{t('ledger.deleted')}</span>}
            </span>
            <span className="row-end">
              <Amount minor={total} currency={from} />
            </span>
          </li>
        ))}
        {lossySettlements.map(({ id, amount, currency: from, deletedAt }) => (
          <li key={id} className="row" style={{ gridTemplateColumns: 'minmax(0, 1fr) auto', minHeight: 0 }}>
            <span className="row-main">
              <span className="row-title">{t('ledger.settlementLabel')}</span>
              {deletedAt && <span className="row-sub">{t('ledgerSettings.settlementDeleted')}</span>}
            </span>
            <span className="row-end">
              <Amount minor={amount} currency={from} />
            </span>
          </li>
        ))}
      </ul>
      <p className="hint">{t('ledgerSettings.lossyHint')}</p>
    </div>
  )
}

function BalanceOf({ balances, mine }: { balances: Balance[]; mine: boolean }) {
  const { t } = useTranslation()
  if (balances.length === 0) return <span className="row-sub">{mine ? t('balance.youSettledShort') : t('balance.memberSettled')}</span>
  return (
    <span className="row-end">
      {balances.map(({ amount, currency }) => (
        <span key={currency} className="row-balance">
          <span className={`row-balance-label tone-${toneOf(amount)}`}>
            {amount > 0 ? t(mine ? 'balance.youOwedShort' : 'balance.memberOwed') : t(mine ? 'balance.youOweShort' : 'balance.memberOwes')}
          </span>
          <Amount minor={Math.abs(amount)} currency={currency} tone={toneOf(amount)} />
        </span>
      ))}
    </span>
  )
}

type MemberAction = 'edit' | 'remove' | 'unlink'

function MemberRow({ groupId, member, mine, balances }: { groupId: string; member: Member; mine: boolean; balances: Balance[] }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [action, setAction] = useState<MemberAction>()
  const [person, setPerson] = useState(nobody)
  const [errors, setErrors] = useState<PersonErrors>()
  const { busy, error, run, reset } = useOperation()
  const pending = !member.userId
  const zero = balances.length === 0
  const name = member.name
  const offlineTitle = online ? undefined : t('common.onlineOnly')

  const open = (next: MemberAction) => {
    reset()
    setErrors(undefined)
    setPerson({ name, email: member.email ?? '' })
    setAction(next)
  }

  const saveEdit = () => {
    const found = checkPerson(t, person)
    setErrors(found)
    if (found) return
    void run(
      () => ledgers.updatePendingMember(groupId, member.id, pendingMember(person)),
      () => {
        setAction(undefined)
        toast(t('ledgerSettings.memberSaved'))
      },
    )
  }

  const remove = () =>
    void run(
      () => ledgers.removeMember(groupId, member.id),
      () => toast(t('ledgerSettings.removed', { name })),
    )

  const unlink = () =>
    void run(
      () => ledgers.undoClaim(groupId, member.id),
      () => toast(t('ledgerSettings.unlinked', { name })),
    )

  return (
    <li>
      <div className="row">
        <span className="row-glyph">
          <Figure state={pending ? 'pending' : 'linked'} />
        </span>
        <span className="row-main">
          <span className="row-title">{mine ? t('common.you') : name}</span>
          {pending && <span className="row-sub">{member.email ?? t('ledger.pending')}</span>}
        </span>
        <BalanceOf balances={balances} mine={mine} />
      </div>

      {!mine && (
        <div style={underTheName}>
          {action === 'edit' ? (
            <form
              className="form"
              style={{ paddingTop: 0 }}
              noValidate
              onSubmit={(e) => {
                e.preventDefault()
                saveEdit()
              }}
            >
              <PersonFields
                value={person}
                onChange={(next) => {
                  setPerson(next)
                  setErrors(undefined)
                }}
                errors={errors}
                emailHint={member.email ? undefined : t('person.emailHiddenHint')}
              />
              <div className="button-row">
                <button type="submit" className="button button-primary button-small" disabled={busy || !online} title={offlineTitle}>
                  {busy ? t('account.saving') : t('account.save')}
                </button>
                <button type="button" className="button button-quiet button-small" disabled={busy} onClick={() => setAction(undefined)}>
                  {t('common.cancel')}
                </button>
              </div>
            </form>
          ) : action ? (
            <div className="confirm" role="group" aria-label={name}>
              <p>{t(`ledgerSettings.${action === 'unlink' ? 'unlinkConfirm' : pending ? 'removePendingConfirm' : 'removeConfirm'}`, { name })}</p>
              <p className="hint">{t(`ledgerSettings.${action === 'unlink' ? 'unlinkHint' : pending ? 'removePendingHint' : 'removeHint'}`)}</p>
              <div className="button-row">
                <button
                  type="button"
                  className="button button-danger button-small"
                  disabled={busy || !online}
                  title={offlineTitle}
                  onClick={action === 'unlink' ? unlink : remove}
                >
                  {t(`ledgerSettings.${action === 'unlink' ? 'unlink' : pending ? 'removePending' : 'remove'}`)}
                </button>
                <button type="button" className="button button-quiet button-small" disabled={busy} onClick={() => setAction(undefined)}>
                  {t('common.cancel')}
                </button>
              </div>
            </div>
          ) : (
            <>
              <div className="button-row">
                {pending && (
                  <button type="button" className="button button-quiet button-small" disabled={!online} title={offlineTitle} onClick={() => open('edit')}>
                    {t('ledgerSettings.edit')}
                  </button>
                )}
                <button
                  type="button"
                  className="button button-quiet button-small"
                  disabled={!online || !zero}
                  title={offlineTitle}
                  aria-label={t(`ledgerSettings.${pending ? 'removePending' : 'remove'}Label`, { name })}
                  onClick={() => open('remove')}
                >
                  {t(`ledgerSettings.${pending ? 'removePending' : 'remove'}`)}
                </button>
                {!pending && (
                  <button
                    type="button"
                    className="button button-quiet button-small"
                    disabled={!online}
                    title={offlineTitle}
                    aria-label={t('ledgerSettings.unlinkLabel', { name })}
                    onClick={() => open('unlink')}
                  >
                    {t('ledgerSettings.unlink')}
                  </button>
                )}
              </div>
              {!zero && <p className="hint">{t('ledgerSettings.memberNotZero', { name })}</p>}
            </>
          )}
          {error && (
            <p className="notice notice-problem" role="alert">
              {problemText(t, error, {
                nonzero_balance: t('ledgerSettings.memberNotZero', { name }),
                member_not_pending: t('ledgerSettings.memberLinked', { name }),
                member_not_claimed: t('ledgerSettings.memberNotLinked', { name }),
              })}
            </p>
          )}
        </div>
      )}
    </li>
  )
}

// Someone removed from the Group, who cannot come back through the Invite link
// until a Member readmits them. Readmitting does not put them back: they do it.
function RemovedRow({ groupId, member }: { groupId: string; member: Member }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [confirming, setConfirming] = useState(false)
  const { busy, error, run, reset } = useOperation()
  const name = member.name
  const offlineTitle = online ? undefined : t('common.onlineOnly')

  const readmit = () =>
    void run(
      () => ledgers.readmitMember(groupId, member.id),
      () => toast(t('ledgerSettings.readmitted', { name })),
    )

  return (
    <li>
      <div className="row">
        <span className="row-glyph">
          <Figure />
        </span>
        <span className="row-main">
          <span className="row-title">{name}</span>
        </span>
      </div>
      <div style={underTheName}>
        {confirming ? (
          <div className="confirm" role="group" aria-label={name}>
            <p>{t('ledgerSettings.readmitConfirm', { name })}</p>
            <p className="hint">{t('ledgerSettings.readmitHint')}</p>
            <div className="button-row">
              <button type="button" className="button button-primary button-small" disabled={busy || !online} title={offlineTitle} onClick={readmit}>
                {t('ledgerSettings.readmit')}
              </button>
              <button type="button" className="button button-quiet button-small" disabled={busy} onClick={() => setConfirming(false)}>
                {t('common.cancel')}
              </button>
            </div>
          </div>
        ) : (
          <div className="button-row">
            <button
              type="button"
              className="button button-quiet button-small"
              disabled={!online}
              title={offlineTitle}
              aria-label={t('ledgerSettings.readmitLabel', { name })}
              onClick={() => {
                reset()
                setConfirming(true)
              }}
            >
              {t('ledgerSettings.readmit')}
            </button>
          </div>
        )}
        {error && (
          <p className="notice notice-problem" role="alert">
            {problemText(t, error, { member_not_removed: t('ledgerSettings.readmitted', { name }) })}
          </p>
        )}
      </div>
    </li>
  )
}

function AddPendingMember({ groupId }: { groupId: string }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [opened, setOpened] = useState(false)
  const [person, setPerson] = useState<Person>(nobody)
  const [errors, setErrors] = useState<PersonErrors>()
  const { busy, error, run } = useOperation()
  const offlineTitle = online ? undefined : t('common.onlineOnly')

  const close = () => {
    setPerson(nobody)
    setErrors(undefined)
    setOpened(false)
  }

  if (!opened) {
    return (
      <button type="button" className="button button-secondary" disabled={!online} title={offlineTitle} onClick={() => setOpened(true)}>
        <Icon name="plus" />
        {t('ledgerSettings.addPending')}
      </button>
    )
  }

  const add = () => {
    const found = checkPerson(t, person)
    setErrors(found)
    if (found) return
    void run(
      () => ledgers.addPendingMember(groupId, pendingMember(person)),
      () => {
        toast(t('ledgerSettings.addedPending', { name: person.name.trim() }))
        close()
      },
    )
  }

  return (
    <form
      className="form"
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        add()
      }}
    >
      <PersonFields
        value={person}
        onChange={(next) => {
          setPerson(next)
          setErrors(undefined)
        }}
        errors={errors}
      />
      {error && (
        <p className="notice notice-problem" role="alert">
          {problemText(t, error)}
        </p>
      )}
      <div className="button-row">
        <button type="submit" className="button button-primary" disabled={busy || !online} title={offlineTitle}>
          {busy ? t('ledgerSettings.adding') : t('ledgerSettings.addPending')}
        </button>
        <button type="button" className="button button-quiet" disabled={busy} onClick={close}>
          {t('common.cancel')}
        </button>
      </div>
    </form>
  )
}

function LeaveOrDelete({ group, me, members, myBalances, settled }: { group: Group; me: Member; members: Member[]; myBalances: Balance[]; settled: boolean }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [action, setAction] = useState<'leave' | 'delete'>()
  const { busy, error, run, reset } = useOperation()
  const offlineTitle = online ? undefined : t('common.onlineOnly')
  const balancesRoute = href({ name: 'ledger', ledgerId: group.id, tab: 'balances' })
  const iAmZero = myBalances.length === 0
  const last = !members.some((member) => member.userId && member.id !== me.id)
  const anyPending = members.some((member) => !member.userId)

  const gone = (text: string) => {
    toast(text)
    navigate({ name: 'home' }, { replace: true })
  }

  const leave = () =>
    void run(
      () => ledgers.leaveGroup(group.id),
      () => gone(t('ledgerSettings.left', { name: group.name })),
    )

  const remove = () =>
    void run(
      () => ledgers.deleteGroup(group.id),
      () => gone(t('ledgerSettings.deleted', { name: group.name })),
    )

  const open = (next: 'leave' | 'delete') => {
    reset()
    setAction(next)
  }

  return (
    <div className="danger-zone">
      {action ? (
        <div className="confirm" role="group" aria-label={group.name}>
          <p>{t(action === 'leave' ? 'ledgerSettings.leaveConfirm' : 'ledgerSettings.deleteConfirm', { name: group.name })}</p>
          <p className="hint">
            {action === 'delete'
              ? t('ledgerSettings.deleteHint')
              : last
                ? t(anyPending ? 'ledgerSettings.leaveLastPending' : 'ledgerSettings.leaveLastAlone')
                : t('ledgerSettings.leaveHint')}
          </p>
          <div className="button-row">
            <button type="button" className="button button-danger button-small" disabled={busy || !online} title={offlineTitle} onClick={action === 'leave' ? leave : remove}>
              {t(action === 'leave' ? 'ledgerSettings.leaveYes' : 'ledgerSettings.deleteYes')}
            </button>
            <button type="button" className="button button-quiet button-small" disabled={busy} onClick={() => setAction(undefined)}>
              {t('common.cancel')}
            </button>
          </div>
        </div>
      ) : (
        <>
          <button type="button" className="button button-quiet" disabled={!online || !iAmZero} title={offlineTitle} onClick={() => open('leave')}>
            {t('ledgerSettings.leave')}
          </button>
          {!iAmZero && (
            <p className="hint">
              {t('ledgerSettings.leaveNotZero')} <a href={balancesRoute}>{t('ledgerSettings.seeBalances')}</a>
            </p>
          )}
          <button type="button" className="button button-quiet" disabled={!online || !settled} title={offlineTitle} onClick={() => open('delete')}>
            {t('ledgerSettings.delete')}
          </button>
          {!settled && (
            <p className="hint">
              {t('ledgerSettings.deleteNotZero')} <a href={balancesRoute}>{t('ledgerSettings.seeBalances')}</a>
            </p>
          )}
          {!online && <p className="hint">{t('common.onlineOnly')}</p>}
        </>
      )}
      {error && (
        <p className="notice notice-problem" role="alert">
          {problemText(t, error, { nonzero_balance: t(action === 'delete' ? 'ledgerSettings.deleteNotZero' : 'ledgerSettings.leaveNotZero') })}
        </p>
      )}
    </div>
  )
}
