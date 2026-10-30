package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestClaimLinksAPendingMemberToAUser(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	insertUser(t, db, "beto")
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		pending := newMember("member-beto", "ledger-1", nil, version)
		pending.Email = ptr("beto@example.com")
		return q.CreateMember(ctx, pending)
	})

	var claimed store.Member
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		claimed, err = q.ClaimMember(ctx, store.ClaimMemberParams{ID: "member-beto", UserID: "beto", Version: version})
		return err
	})

	if claimed.UserID == nil || *claimed.UserID != "beto" || claimed.Version != version {
		t.Errorf("claimed member = %+v, want linked to beto with version %d", claimed, version)
	}
}

func TestClaimRefusesAMemberThatIsAlreadyLinked(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	insertUser(t, db, "ana")
	insertUser(t, db, "beto")
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("member-ana", "ledger-1", ptr("ana"), version))
	})

	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		_, err := q.ClaimMember(ctx, store.ClaimMemberParams{ID: "member-ana", UserID: "beto", Version: version})
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("ClaimMember on a linked member: err = %v, want sql.ErrNoRows", err)
	}
}

func TestUnclaimTurnsAMemberBackIntoAPendingOne(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	insertUser(t, db, "beto")
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("member-beto", "ledger-1", ptr("beto"), version))
	})

	var unclaimed store.Member
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		unclaimed, err = q.UnclaimMember(ctx, store.UnclaimMemberParams{ID: "member-beto", Version: version})
		return err
	})

	if unclaimed.UserID != nil || unclaimed.Name != "member-beto" || unclaimed.Version != version {
		t.Errorf("unclaimed member = %+v, want pending with its name and version %d", unclaimed, version)
	}
}

func TestDeletingAnAccountLeavesAllItsMembersPending(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	insertUser(t, db, "ana")
	insertUser(t, db, "beto")
	write(t, db, func(q *store.Queries, version int64) error {
		for _, ledger := range []string{"ledger-1", "ledger-2"} {
			if err := q.CreateLedger(ctx, newGroup(ledger, version)); err != nil {
				return err
			}
			if err := q.CreateMember(ctx, newMember("ana-in-"+ledger, ledger, ptr("ana"), version)); err != nil {
				return err
			}
			if err := q.CreateMember(ctx, newMember("beto-in-"+ledger, ledger, ptr("beto"), version)); err != nil {
				return err
			}
		}
		return nil
	})

	version := write(t, db, func(q *store.Queries, version int64) error {
		return q.UnlinkMembersOfUser(ctx, store.UnlinkMembersOfUserParams{UserID: "ana", Version: version})
	})

	for _, ledger := range []string{"ledger-1", "ledger-2"} {
		members, err := store.New(db).ListMembers(ctx, ledger)
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 2 {
			t.Fatalf("ListMembers(%s) returned %d members, want 2", ledger, len(members))
		}
		for _, m := range members {
			switch {
			case m.ID == "ana-in-"+ledger && (m.UserID != nil || m.Version != version):
				t.Errorf("Ana's member = %+v, want pending with version %d", m, version)
			case m.ID == "beto-in-"+ledger && m.UserID == nil:
				t.Errorf("Beto's member was unlinked too: %+v", m)
			}
		}
	}
}

func TestListLedgersOfUserOnlyListsThoseWithAnActiveMember(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	insertUser(t, db, "ana")
	insertUser(t, db, "beto")
	write(t, db, func(q *store.Queries, version int64) error {
		for _, ledger := range []string{"active", "left", "deleted", "someone-else"} {
			if err := q.CreateLedger(ctx, newGroup(ledger, version)); err != nil {
				return err
			}
		}
		for _, m := range []store.CreateMemberParams{
			newMember("ana-active", "active", ptr("ana"), version),
			newMember("ana-left", "left", ptr("ana"), version),
			newMember("ana-deleted", "deleted", ptr("ana"), version),
			newMember("beto-someone-else", "someone-else", ptr("beto"), version),
		} {
			if err := q.CreateMember(ctx, m); err != nil {
				return err
			}
		}
		return nil
	})
	write(t, db, func(q *store.Queries, version int64) error {
		if _, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: "ana-left", Version: version}); err != nil {
			return err
		}
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "deleted", DeletedAt: &now, Version: version})
		return err
	})

	ledgers, err := store.New(db).ListLedgersOfUser(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != 1 || ledgers[0].ID != "active" {
		t.Errorf("ListLedgersOfUser(ana) = %+v, want only the ledger she is still active in", ledgers)
	}
}

func TestGetActiveMemberOfUserFindsWhoTheUserIsInALedger(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	insertUser(t, db, "ana")
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("member-ana", "ledger-1", ptr("ana"), version))
	})
	params := store.GetActiveMemberOfUserParams{LedgerID: "ledger-1", UserID: "ana"}

	if m, err := q.GetActiveMemberOfUser(ctx, params); err != nil || m.ID != "member-ana" {
		t.Errorf("GetActiveMemberOfUser = %q, %v; want member-ana", m.ID, err)
	}

	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: "member-ana", Version: version})
		return err
	})
	if _, err := q.GetActiveMemberOfUser(ctx, params); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetActiveMemberOfUser after leaving: err = %v, want sql.ErrNoRows", err)
	}
}

func TestReactivateMemberBringsAFormerMemberBack(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: "member-beto", Version: version})
		return err
	})

	var back store.Member
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		back, err = q.ReactivateMember(ctx, store.ReactivateMemberParams{ID: "member-beto", Version: version})
		return err
	})

	if back.Status != "active" || back.UserID == nil || *back.UserID != "beto" || back.Version != version {
		t.Errorf("reactivated member = %+v, want Beto active again with version %d", back, version)
	}
	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		_, err := q.ReactivateMember(ctx, store.ReactivateMemberParams{ID: "member-ana", Version: version})
		return err
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("ReactivateMember on an active member: err = %v, want sql.ErrNoRows", err)
	}
}

func TestGetUserSharingAGroupNeedsBothActiveInALiveGroup(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	seedGroup(t, db)
	insertUser(t, db, "carla")
	params := store.GetUserSharingAGroupParams{UserID: "ana", OtherUserID: "beto"}

	if u, err := q.GetUserSharingAGroup(ctx, params); err != nil || u.ID != "beto" || u.Name != "beto" {
		t.Errorf("GetUserSharingAGroup = %+v, %v; want Beto", u, err)
	}
	stranger := store.GetUserSharingAGroupParams{UserID: "ana", OtherUserID: "carla"}
	if _, err := q.GetUserSharingAGroup(ctx, stranger); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUserSharingAGroup with a stranger: err = %v, want sql.ErrNoRows", err)
	}

	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: "member-beto", Version: version})
		return err
	})
	if _, err := q.GetUserSharingAGroup(ctx, params); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUserSharingAGroup after Beto left: err = %v, want sql.ErrNoRows", err)
	}

	write(t, db, func(q *store.Queries, version int64) error {
		if _, err := q.ReactivateMember(ctx, store.ReactivateMemberParams{ID: "member-beto", Version: version}); err != nil {
			return err
		}
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "ledger-1", DeletedAt: &now, Version: version})
		return err
	})
	if _, err := q.GetUserSharingAGroup(ctx, params); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUserSharingAGroup in a deleted Group: err = %v, want sql.ErrNoRows", err)
	}
}

func TestGetFriendshipBetweenFindsItFromEitherSide(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	// ana and beto also share a Group, which is not a Friendship.
	seedGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, store.CreateLedgerParams{ID: "friends", Kind: "friendship", SimplifyDebts: true, Version: version, CreatedAt: now}); err != nil {
			return err
		}
		if err := q.CreateMember(ctx, newMember("friend-ana", "friends", ptr("ana"), version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("friend-beto", "friends", ptr("beto"), version))
	})
	insertUser(t, db, "carla")

	for _, pair := range [][2]string{{"ana", "beto"}, {"beto", "ana"}} {
		got, err := q.GetFriendshipBetween(ctx, store.GetFriendshipBetweenParams{UserID: pair[0], OtherUserID: pair[1]})
		if err != nil || got.ID != "friends" {
			t.Errorf("GetFriendshipBetween(%s, %s) = %q, %v; want friends", pair[0], pair[1], got.ID, err)
		}
	}
	_, err := q.GetFriendshipBetween(ctx, store.GetFriendshipBetweenParams{UserID: "ana", OtherUserID: "carla"})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetFriendshipBetween without one: err = %v, want sql.ErrNoRows", err)
	}
}
