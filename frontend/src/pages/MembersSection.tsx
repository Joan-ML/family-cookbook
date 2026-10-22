import { useState, type FormEvent } from 'react'
import { useLocation } from 'wouter'
import type { Cookbook, Member, Role } from '../api/client'
import { addMember, changeRole, removeMember } from '../api/members'
import { paths } from '../paths'

const roleLabel: Record<Role, string> = { admin: 'Admin', editor: 'Editor' }

/**
 * Who's in the cookbook. Everyone sees it and can leave; admins also add
 * people, change roles and remove members. The server refuses changes that
 * would leave the cookbook without an admin, and its message is shown.
 */
export function MembersSection({ cookbook, me, loaded }: { cookbook: Cookbook; me: string; loaded: Member[] }) {
  const [members, setMembers] = useState(loaded)
  const [error, setError] = useState('')
  const isAdmin = cookbook.role === 'admin'

  async function attempt(action: () => Promise<void>) {
    setError('')
    try {
      await action()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const onRole = (m: Member, role: Role) =>
    attempt(async () => {
      const updated = await changeRole(cookbook.id, m.username, role)
      setMembers((ms) => ms.map((x) => (x.username === updated.username ? updated : x)))
    })

  const onRemove = (m: Member) =>
    attempt(async () => {
      if (!window.confirm(`Remove ${m.username} from "${cookbook.name}"?`)) return
      await removeMember(cookbook.id, m.username)
      setMembers((ms) => ms.filter((x) => x.username !== m.username))
    })

  return (
    <section className="settings-section">
      <h2>Members</h2>
      <ul className="members" aria-label="Members">
        {members.map((m) => (
          <li key={m.username}>
            <span className="member-name">
              {m.username}
              {m.username === me && <span className="muted"> (you)</span>}
            </span>
            {isAdmin ? (
              <>
                <select
                  aria-label={`Role of ${m.username}`}
                  value={m.role}
                  onChange={(e) => onRole(m, e.target.value as Role)}
                >
                  <option value="admin">Admin</option>
                  <option value="editor">Editor</option>
                </select>
                {m.username !== me && (
                  <button
                    type="button"
                    className="link-button"
                    aria-label={`Remove ${m.username}`}
                    onClick={() => onRemove(m)}
                  >
                    Remove
                  </button>
                )}
              </>
            ) : (
              <span className="muted">{roleLabel[m.role]}</span>
            )}
          </li>
        ))}
      </ul>
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      {isAdmin ? (
        <AddMember cookbook={cookbook} onAdded={(m) => setMembers((ms) => sortMembers([...ms, m]))} />
      ) : (
        <p className="muted">Only the cookbook's admins can add or remove members.</p>
      )}
      <Leave cookbook={cookbook} me={me} />
    </section>
  )
}

const sortMembers = (ms: Member[]) => [...ms].sort((a, b) => a.username.localeCompare(b.username))

function AddMember({ cookbook, onAdded }: { cookbook: Cookbook; onAdded: (m: Member) => void }) {
  const [username, setUsername] = useState('')
  const [role, setRole] = useState<Role>('editor')
  const [error, setError] = useState('')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      onAdded(await addMember(cookbook.id, username, role))
      setUsername('')
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <form className="add-member" onSubmit={onSubmit}>
      <h3>Add someone</h3>
      <small className="muted">They need an account; ask whoever runs the server to create one.</small>
      <div className="inline-field">
        <input
          aria-label="Username to add"
          placeholder="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          required
        />
        <select aria-label="Role for the new member" value={role} onChange={(e) => setRole(e.target.value as Role)}>
          <option value="editor">Editor</option>
          <option value="admin">Admin</option>
        </select>
        <button type="submit" className="button primary">
          Add
        </button>
      </div>
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
    </form>
  )
}

function Leave({ cookbook, me }: { cookbook: Cookbook; me: string }) {
  const [, navigate] = useLocation()
  const [error, setError] = useState('')

  async function onLeave() {
    if (!window.confirm(`Leave "${cookbook.name}"? You'll no longer see its recipes.`)) return
    setError('')
    try {
      await removeMember(cookbook.id, me)
      navigate(paths.home, { replace: true })
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div className="leave">
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <button type="button" className="button" onClick={onLeave}>
        Leave cookbook
      </button>
    </div>
  )
}
