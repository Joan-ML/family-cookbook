import { Link } from 'wouter'

export function NotFound({ what = 'page' }: { what?: string }) {
  return (
    <div className="empty">
      <h1>Not found</h1>
      <p>
        This {what} doesn't exist. It may have been deleted. <Link href="/">Back to all recipes</Link>
      </p>
    </div>
  )
}
