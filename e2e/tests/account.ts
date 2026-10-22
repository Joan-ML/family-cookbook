// The accounts the tests use. Both must exist on the server under test, e.g.
//   cookbook user add e2e <<< 'e2e test password'
//   cookbook user add e2e-friend <<< 'e2e friend password'
export const account = {
  username: process.env.E2E_USERNAME ?? 'e2e',
  password: process.env.E2E_PASSWORD ?? 'e2e test password',
}

/** A second person, for testing sharing. */
export const friend = {
  username: process.env.E2E_FRIEND_USERNAME ?? 'e2e-friend',
  password: process.env.E2E_FRIEND_PASSWORD ?? 'e2e friend password',
}
