// The account the tests use. It must exist on the server under test, e.g.
//   cookbook user add e2e <<< 'e2e test password'
export const account = {
  username: process.env.E2E_USERNAME ?? 'e2e',
  password: process.env.E2E_PASSWORD ?? 'e2e test password',
}
