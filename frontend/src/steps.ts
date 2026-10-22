/** A step being edited. `id` stays the same while steps are reordered. */
export type Step = { id: string; text: string }

let lastId = 0
export function newStep(text = ''): Step {
  return { id: `step-${++lastId}`, text }
}

// Matches the numbering recipes copied from websites often start steps with:
// "1.", "2)", "Step 3", "Step 4:".
const numbering = /^\s*(?:step\s*\d+\s*[:.)-]?|\d+\s*[.)])\s*/i

/**
 * Splits pasted text into steps, or returns null when it's a single step.
 *
 * Steps are separated by blank lines. Text without blank lines is split
 * where lines start with step numbering instead, since copying a list from a
 * web page often loses the blank lines. Numbering is removed: the form numbers
 * steps itself.
 */
export function splitSteps(text: string): string[] | null {
  const normalized = text.replace(/\r\n?/g, '\n').trim()
  let parts: string[]
  if (/\n[ \t]*\n/.test(normalized)) {
    parts = normalized.split(/\n[ \t]*\n/)
  } else {
    const lines = normalized.split('\n')
    if (lines.filter((l) => numbering.test(l)).length < 2 || !numbering.test(lines[0])) return null
    parts = []
    for (const line of lines) {
      if (numbering.test(line) || parts.length === 0) parts.push(line)
      else parts[parts.length - 1] += '\n' + line // a wrapped line continues its step
    }
  }
  parts = parts.map((p) => p.replace(numbering, '').trim()).filter(Boolean)
  return parts.length > 1 ? parts : null
}
