import type { Friendship, Group, Invite, InviteLink, JoinedLedger, Member } from '../api/client'
import type { components } from '../api/schema.gen'
import { type ApiClient, call, type CallResult } from './api'

// What creates and manages Groups and Friendships. All of it needs the network
// (ADR 0001): it goes straight to the API and the replica follows with a sync.

type Schemas = components['schemas']
export type NewGroup = Schemas['NewGroup']
export type GroupUpdate = Schemas['GroupUpdate']
export type NewPendingMember = Schemas['NewPendingMember']
export type NewFriendship = Schemas['NewFriendship']

// Both ledgers keep their own Invite link. Only a Friendship with a pending
// Friend has one to show.
export interface InviteLinkOwner {
  id: string
  kind: 'group' | 'friendship'
}

export type LedgerCalls = ReturnType<typeof createLedgerCalls>

export function createLedgerCalls({ client, sync }: { client: ApiClient; sync: () => Promise<void> }) {
  // The replica catches up before the caller goes on, so the next screen finds
  // the change. A refusal syncs too: it means the replica is behind the server.
  async function change<T>(send: () => Promise<CallResult<T>>): Promise<CallResult<T>> {
    const result = await send()
    if (result.ok || result.error.kind === 'problem') await sync()
    return result
  }

  const group = (groupId: string) => ({ params: { path: { groupId } } })
  const member = (groupId: string, memberId: string) => ({ params: { path: { groupId, memberId } } })
  const friendship = (friendshipId: string) => ({ params: { path: { friendshipId } } })
  const invite = (token: string) => ({ params: { path: { token } } })

  return {
    createGroup: (body: NewGroup): Promise<CallResult<Group>> => change(() => call(() => client.POST('/groups', { body }))),

    updateGroup: (groupId: string, body: GroupUpdate): Promise<CallResult<Group>> =>
      change(() => call(() => client.PATCH('/groups/{groupId}', { ...group(groupId), body }))),

    deleteGroup: (groupId: string): Promise<CallResult<void>> => change(() => call(() => client.DELETE('/groups/{groupId}', group(groupId)))),

    leaveGroup: (groupId: string): Promise<CallResult<void>> => change(() => call(() => client.POST('/groups/{groupId}/leave', group(groupId)))),

    addPendingMember: (groupId: string, body: NewPendingMember): Promise<CallResult<Member>> =>
      change(() => call(() => client.POST('/groups/{groupId}/members', { ...group(groupId), body }))),

    updatePendingMember: (groupId: string, memberId: string, body: NewPendingMember): Promise<CallResult<Member>> =>
      change(() => call(() => client.PUT('/groups/{groupId}/members/{memberId}', { ...member(groupId, memberId), body }))),

    removeMember: (groupId: string, memberId: string): Promise<CallResult<void>> =>
      change(() => call(() => client.DELETE('/groups/{groupId}/members/{memberId}', member(groupId, memberId)))),

    undoClaim: (groupId: string, memberId: string): Promise<CallResult<void>> =>
      change(() => call(() => client.DELETE('/groups/{groupId}/members/{memberId}/claim', member(groupId, memberId)))),

    // Lets someone removed come back through the Invite link; it does not put them back.
    readmitMember: (groupId: string, memberId: string): Promise<CallResult<void>> =>
      change(() => call(() => client.POST('/groups/{groupId}/members/{memberId}/readmit', member(groupId, memberId)))),

    createFriendship: (body: NewFriendship): Promise<CallResult<Friendship>> => change(() => call(() => client.POST('/friendships', { body }))),

    // Showing the link changes nothing, so it does not sync.
    inviteLink: ({ id, kind }: InviteLinkOwner): Promise<CallResult<InviteLink>> =>
      call(() => (kind === 'group' ? client.GET('/groups/{groupId}/invite-link', group(id)) : client.GET('/friendships/{friendshipId}/invite-link', friendship(id)))),

    regenerateInviteLink: ({ id, kind }: InviteLinkOwner): Promise<CallResult<InviteLink>> =>
      change(() =>
        call(() => (kind === 'group' ? client.POST('/groups/{groupId}/invite-link', group(id)) : client.POST('/friendships/{friendshipId}/invite-link', friendship(id)))),
      ),

    getInvite: (token: string): Promise<CallResult<Invite>> => call(() => client.GET('/invites/{token}', invite(token))),

    claimMember: (token: string, memberId: string): Promise<CallResult<JoinedLedger>> =>
      change(() => call(() => client.POST('/invites/{token}/claim', { ...invite(token), body: { memberId } }))),

    joinGroup: (token: string): Promise<CallResult<JoinedLedger>> => change(() => call(() => client.POST('/invites/{token}/join', invite(token)))),
  }
}
