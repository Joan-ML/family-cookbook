import { Link } from 'wouter'

/** Tags as small chips; each links to the list filtered by that tag. */
export function TagList({ tags }: { tags: string[] }) {
  if (tags.length === 0) return null
  return (
    <ul className="tags" aria-label="Tags">
      {tags.map((tag) => (
        <li key={tag}>
          <Link className="tag" href={`/?tag=${encodeURIComponent(tag)}`}>
            {tag}
          </Link>
        </li>
      ))}
    </ul>
  )
}

export function Loading() {
  return (
    <p className="muted" role="status">
      Loading…
    </p>
  )
}

export function LoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="alert" role="alert">
      <p>{message}</p>
      <button type="button" className="button" onClick={onRetry}>
        Try again
      </button>
    </div>
  )
}
