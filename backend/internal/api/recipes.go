package api

import (
	"context"
	"errors"
	"slices"
	"strings"

	"family-cookbook/backend/internal/store"
)

// Every recipe handler first checks that the current user is a member of
// the cookbook in the URL; for anyone else, the cookbook doesn't exist.

func (s *server) ListRecipes(ctx context.Context, req ListRecipesRequestObject) (ListRecipesResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return ListRecipes404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	summaries, err := s.store.ListRecipes(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	items := make([]RecipeSummary, len(summaries))
	for i, sum := range summaries {
		items[i] = RecipeSummary{Id: sum.ID, Name: sum.Name, Tags: sum.Tags}
	}
	return ListRecipes200JSONResponse{Items: items}, nil
}

func (s *server) CreateRecipe(ctx context.Context, req CreateRecipeRequestObject) (CreateRecipeResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return CreateRecipe404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	content, msg := normalize(*req.Body)
	if msg != "" {
		return CreateRecipe400JSONResponse{BadRequestJSONResponse{Message: msg}}, nil
	}

	now, user := s.now().UTC(), currentUser(ctx)
	r := store.Recipe{
		ID:            s.newID(),
		CookbookID:    c.ID,
		RecipeContent: content,
		CreatedAt:     now,
		CreatedBy:     user,
		UpdatedAt:     now,
		UpdatedBy:     user,
	}
	if err := s.store.CreateRecipe(ctx, r); err != nil {
		return nil, err
	}

	location := basePath + "/cookbooks/" + c.ID + "/recipes/" + r.ID
	return CreateRecipe201JSONResponse{
		Body:    toAPIRecipe(r),
		Headers: CreateRecipe201ResponseHeaders{Location: &location},
	}, nil
}

func (s *server) GetRecipe(ctx context.Context, req GetRecipeRequestObject) (GetRecipeResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return GetRecipe404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := s.store.GetRecipe(ctx, c.ID, req.RecipeId)
	if errors.Is(err, store.ErrNotFound) {
		return GetRecipe404JSONResponse{recipeNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return GetRecipe200JSONResponse(toAPIRecipe(r)), nil
}

func (s *server) UpdateRecipe(ctx context.Context, req UpdateRecipeRequestObject) (UpdateRecipeResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return UpdateRecipe404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	content, msg := normalize(*req.Body)
	if msg != "" {
		return UpdateRecipe400JSONResponse{BadRequestJSONResponse{Message: msg}}, nil
	}

	r, err := s.store.UpdateRecipe(ctx, c.ID, req.RecipeId, content, currentUser(ctx), s.now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		return UpdateRecipe404JSONResponse{recipeNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return UpdateRecipe200JSONResponse(toAPIRecipe(r)), nil
}

func (s *server) DeleteRecipe(ctx context.Context, req DeleteRecipeRequestObject) (DeleteRecipeResponseObject, error) {
	c, err := s.memberCookbook(ctx, req.CookbookId)
	if errors.Is(err, errNotMember) {
		return DeleteRecipe404JSONResponse{cookbookNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	err = s.store.DeleteRecipe(ctx, c.ID, req.RecipeId)
	if errors.Is(err, store.ErrNotFound) {
		return DeleteRecipe404JSONResponse{recipeNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return DeleteRecipe204Response{}, nil
}

var recipeNotFound = NotFoundJSONResponse{Message: "recipe not found"}

// normalize trims user input and applies the rules the spec can't express:
// a name can't be blank, empty lines are dropped, and tags are lowercased and
// de-duplicated. It returns a message if the input is invalid.
func normalize(in RecipeInput) (store.RecipeContent, string) {
	c := store.RecipeContent{
		Name:        strings.TrimSpace(in.Name),
		Ingredients: trimLines(in.Ingredients),
		Steps:       trimLines(in.Steps),
		Tags:        []string{},
	}
	if c.Name == "" {
		return c, "name must not be blank"
	}
	if in.Source != nil {
		c.Source = strings.TrimSpace(*in.Source)
	}
	if in.Servings != nil {
		c.Servings = *in.Servings // the spec limits it to 1–100
	}
	for _, tag := range trimLines(in.Tags) {
		tag = strings.ToLower(tag)
		if !slices.Contains(c.Tags, tag) {
			c.Tags = append(c.Tags, tag)
		}
	}
	return c, ""
}

func trimLines(lines []string) []string {
	out := []string{}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func toAPIRecipe(r store.Recipe) Recipe {
	out := Recipe{
		Id:          r.ID,
		Name:        r.Name,
		Ingredients: r.Ingredients,
		Steps:       r.Steps,
		Tags:        r.Tags,
		CreatedAt:   r.CreatedAt,
		CreatedBy:   r.CreatedBy,
		UpdatedAt:   r.UpdatedAt,
		UpdatedBy:   r.UpdatedBy,
	}
	if r.Source != "" {
		out.Source = &r.Source
	}
	if r.Servings > 0 {
		out.Servings = &r.Servings
	}
	return out
}
