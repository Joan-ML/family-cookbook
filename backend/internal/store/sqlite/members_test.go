package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"family-cookbook/backend/internal/store"
)

func TestMembersAndRoles(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	owner := addUser(t, s, "owner")
	anna := addUser(t, s, "anna")
	addCookbook(t, s, "c1", "Family", owner)
	must(t, s.AddMember(ctx, "c1", anna, store.RoleEditor, time.Now()))

	members, err := s.ListMembers(ctx, "c1")
	if err != nil || len(members) != 2 || members[0].Username != "anna" || members[1].Username != "owner" ||
		members[0].Role != store.RoleEditor || members[1].Role != store.RoleAdmin {
		t.Fatalf("members = %+v, %v", members, err)
	}

	// The last admin can't step down or leave...
	if _, err := s.SetRole(ctx, "c1", owner, store.RoleEditor); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("demoting the last admin: err = %v", err)
	}
	if err := s.RemoveMember(ctx, "c1", owner); !errors.Is(err, store.ErrLastAdmin) {
		t.Errorf("removing the last admin: err = %v", err)
	}
	// ...until someone else is admin.
	m, err := s.SetRole(ctx, "c1", anna, store.RoleAdmin)
	if err != nil || m.Username != "anna" || m.Role != store.RoleAdmin {
		t.Fatalf("promoting anna: %+v, %v", m, err)
	}
	must(t, s.RemoveMember(ctx, "c1", owner))
	if _, err := s.GetCookbook(ctx, "c1", owner); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("owner still sees the cookbook after leaving: %v", err)
	}

	if _, err := s.SetRole(ctx, "c1", owner, store.RoleEditor); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("changing a non-member's role: err = %v", err)
	}
	if err := s.RemoveMember(ctx, "c1", owner); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing a non-member: err = %v", err)
	}
}

// Two admins demoting each other at the same moment must not leave the
// cookbook without an admin: exactly one change wins, the other is refused.
func TestConcurrentDemotionsKeepAnAdmin(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	a := addUser(t, s, "a")
	b := addUser(t, s, "b")

	for round := range 20 {
		id := "c" + string(rune('A'+round))
		addCookbook(t, s, id, "Shared", a)
		must(t, s.AddMember(ctx, id, b, store.RoleAdmin, time.Now()))

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, user := range []int64{a, b} {
			wg.Go(func() { _, errs[i] = s.SetRole(ctx, id, user, store.RoleEditor) })
		}
		wg.Wait()

		lastAdmin := 0
		for _, err := range errs {
			switch {
			case errors.Is(err, store.ErrLastAdmin):
				lastAdmin++
			case err != nil:
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		members, _ := s.ListMembers(ctx, id)
		admins := 0
		for _, m := range members {
			if m.Role == store.RoleAdmin {
				admins++
			}
		}
		if lastAdmin != 1 || admins != 1 {
			t.Fatalf("round %d: %d refused, %d admins left; want exactly 1 and 1 (errs %v)", round, lastAdmin, admins, errs)
		}
	}
}
