package api

import (
	"context"
	"errors"
	"strings"

	"family-cookbook/backend/internal/store"
)

var (
	cookbookNotFound = NotFoundJSONResponse{Message: "cookbook not found"}
	adminsOnly       = ForbiddenJSONResponse{Message: "only the cookbook's admins can do this"}
)

// errNotMember is what non-members get for a cookbook: the same as a
// cookbook that doesn't exist, so cookbooks can't be discovered.
var errNotMember = store.ErrNotFound

// memberCookbook returns the cookbook if the current user is a member, or
// errNotMember.
func (s *server) memberCookbook(ctx context.Context, id string) (store.Cookbook, error) {
	return s.store.GetCookbook(ctx, id, currentUserID(ctx))
}

func (s *server) ListCookbooks(ctx context.Context, _ ListCookbooksRequestObject) (ListCookbooksResponseObject, error) {
	cookbooks, err := s.store.ListCookbooks(ctx, currentUserID(ctx))
	if err != nil {
		return nil, err
	}
	items := make([]CookbookSummary, len(cookbooks))
	for i, c := range cookbooks {
		items[i] = CookbookSummary{Id: c.ID, Name: c.Name, Role: Role(c.Role), RecipeCount: c.RecipeCount}
	}
	return ListCookbooks200JSONResponse{Items: items}, nil
}

func (s *server) CreateCookbook(ctx context.Context, req CreateCookbookRequestObject) (CreateCookbookResponseObject, error) {
	name := strings.TrimSpace(req.Body.Name)
	if name == "" {
		return CreateCookbook400JSONResponse{BadRequestJSONResponse{Message: "name must not be blank"}}, nil
	}
	c := store.Cookbook{
		ID:        s.newID(),
		Name:      name,
		CreatedAt: s.now().UTC(),
		CreatedBy: currentUser(ctx),
		Role:      store.RoleAdmin,
	}
	if err := s.store.CreateCookbook(ctx, c, currentUserID(ctx)); err != nil {
		return nil, err
	}
	location := basePath + "/cookbooks/" + c.ID
	return CreateCookbook201JSONResponse{
		Body:    toAPICookbook(c),
		Headers: CreateCookbook201ResponseHeaders{Location: &location},
	}, nil
}

func (s *server) GetCookbook(ctx context.Context, req GetCookbookRequestObject) (GetCookbookResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return GetCookbook404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return GetCookbook200JSONResponse(toAPICookbook(c)), nil
}

func (s *server) RenameCookbook(ctx context.Context, req RenameCookbookRequestObject) (RenameCookbookResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return RenameCookbook404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Role != store.RoleAdmin {
		return RenameCookbook403JSONResponse{adminsOnly}, nil
	}
	name := strings.TrimSpace(req.Body.Name)
	if name == "" {
		return RenameCookbook400JSONResponse{BadRequestJSONResponse{Message: "name must not be blank"}}, nil
	}
	if err := s.store.RenameCookbook(ctx, c.ID, name); err != nil {
		return nil, err
	}
	c.Name = name
	return RenameCookbook200JSONResponse(toAPICookbook(c)), nil
}

func (s *server) DeleteCookbook(ctx context.Context, req DeleteCookbookRequestObject) (DeleteCookbookResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return DeleteCookbook404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Role != store.RoleAdmin {
		return DeleteCookbook403JSONResponse{adminsOnly}, nil
	}
	if err := s.store.DeleteCookbook(ctx, c.ID); err != nil {
		return nil, err
	}
	return DeleteCookbook204Response{}, nil
}

func toAPICookbook(c store.Cookbook) Cookbook {
	return Cookbook{Id: c.ID, Name: c.Name, Role: Role(c.Role), CreatedAt: c.CreatedAt, CreatedBy: c.CreatedBy}
}
