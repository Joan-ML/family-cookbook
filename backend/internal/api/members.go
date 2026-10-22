package api

import (
	"context"
	"errors"
	"fmt"

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store"
)

var memberNotFound = NotFoundJSONResponse{Message: "member not found"}

// member looks up a cookbook member by username. ok is false if there's no
// such account or it isn't a member of the cookbook.
func (s *server) member(ctx context.Context, cookbookID, username string) (user store.User, ok bool, err error) {
	name, err := auth.NormalizeUsername(username)
	if err != nil {
		return store.User{}, false, nil
	}
	user, err = s.store.GetUser(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, false, nil
	}
	if err != nil {
		return store.User{}, false, err
	}
	if _, err := s.store.GetCookbook(ctx, cookbookID, user.ID); errors.Is(err, store.ErrNotFound) {
		return store.User{}, false, nil
	} else if err != nil {
		return store.User{}, false, err
	}
	return user, true, nil
}

func (s *server) ListMembers(ctx context.Context, req ListMembersRequestObject) (ListMembersResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return ListMembers404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	items := make([]Member, len(members))
	for i, m := range members {
		items[i] = toAPIMember(m)
	}
	return ListMembers200JSONResponse{Items: items}, nil
}

func (s *server) AddMember(ctx context.Context, req AddMemberRequestObject) (AddMemberResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return AddMember404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Role != store.RoleAdmin {
		return AddMember403JSONResponse{adminsOnly}, nil
	}
	noAccount := AddMember400JSONResponse{Message: fmt.Sprintf("no account named %q", req.Body.Username)}
	name, err := auth.NormalizeUsername(req.Body.Username)
	if err != nil {
		return noAccount, nil
	}
	user, err := s.store.GetUser(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return noAccount, nil
	}
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	err = s.store.AddMember(ctx, c.ID, user.ID, store.Role(req.Body.Role), now)
	if errors.Is(err, store.ErrExists) {
		return AddMember409JSONResponse{ConflictJSONResponse{Message: user.Username + " is already a member"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return AddMember201JSONResponse{Username: user.Username, Role: req.Body.Role, JoinedAt: now}, nil
}

func (s *server) ChangeRole(ctx context.Context, req ChangeRoleRequestObject) (ChangeRoleResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return ChangeRole404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Role != store.RoleAdmin {
		return ChangeRole403JSONResponse{adminsOnly}, nil
	}
	user, ok, err := s.member(ctx, c.ID, req.Username)
	if err != nil {
		return nil, err
	}
	if !ok {
		return ChangeRole404JSONResponse{memberNotFound}, nil
	}
	m, err := s.store.SetRole(ctx, c.ID, user.ID, store.Role(req.Body.Role))
	switch {
	case errors.Is(err, store.ErrLastAdmin):
		return ChangeRole409JSONResponse{ConflictJSONResponse{
			Message: "a cookbook needs at least one admin: make someone else admin first",
		}}, nil
	case errors.Is(err, store.ErrNotFound): // removed in the meantime
		return ChangeRole404JSONResponse{memberNotFound}, nil
	case err != nil:
		return nil, err
	}
	return ChangeRole200JSONResponse(toAPIMember(m)), nil
}

func (s *server) RemoveMember(ctx context.Context, req RemoveMemberRequestObject) (RemoveMemberResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return RemoveMember404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	user, ok, err := s.member(ctx, c.ID, req.Username)
	if err != nil {
		return nil, err
	}
	leaving := ok && user.ID == currentUserID(ctx)
	// Anyone may leave; only admins remove others. Editors learn nothing
	// about non-members: they get 403 either way.
	if !leaving && c.Role != store.RoleAdmin {
		return RemoveMember403JSONResponse{adminsOnly}, nil
	}
	if !ok {
		return RemoveMember404JSONResponse{memberNotFound}, nil
	}
	err = s.store.RemoveMember(ctx, c.ID, user.ID)
	switch {
	case errors.Is(err, store.ErrLastAdmin):
		msg := "a cookbook needs at least one admin: make someone else admin first"
		if leaving {
			msg = "you're this cookbook's only admin: make someone else admin before leaving, or delete the cookbook"
		}
		return RemoveMember409JSONResponse{ConflictJSONResponse{Message: msg}}, nil
	case errors.Is(err, store.ErrNotFound):
		return RemoveMember404JSONResponse{memberNotFound}, nil
	case err != nil:
		return nil, err
	}
	return RemoveMember204Response{}, nil
}

func toAPIMember(m store.Member) Member {
	return Member{Username: m.Username, Role: Role(m.Role), JoinedAt: m.JoinedAt}
}
