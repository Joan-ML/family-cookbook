// The app's addresses, in one place.
const enc = encodeURIComponent

export const paths = {
  home: '/',
  account: '/account',
  setPassword: '/set-password',
  cookbook: (cookbookId: string) => `/cookbooks/${enc(cookbookId)}`,
  cookbookTag: (cookbookId: string, tag: string) => `/cookbooks/${enc(cookbookId)}?tag=${enc(tag)}`,
  cookbookSettings: (cookbookId: string) => `/cookbooks/${enc(cookbookId)}/settings`,
  newRecipe: (cookbookId: string) => `/cookbooks/${enc(cookbookId)}/new`,
  recipe: (cookbookId: string, recipeId: string) => `/cookbooks/${enc(cookbookId)}/recipes/${enc(recipeId)}`,
  editRecipe: (cookbookId: string, recipeId: string) => `/cookbooks/${enc(cookbookId)}/recipes/${enc(recipeId)}/edit`,
}
