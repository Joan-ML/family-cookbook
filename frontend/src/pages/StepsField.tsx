import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { ClipboardEvent } from 'react'
import { newStep, splitSteps, type Step } from '../steps'

/**
 * Ordered step editor. Steps are reordered by dragging their handle (touch or
 * mouse), or from the keyboard: focus a handle, press Space, move with the
 * arrow keys, and press Space again.
 */
export function StepsField({ steps, onChange }: { steps: Step[]; onChange: (steps: Step[]) => void }) {
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const position = (id: string | number) => steps.findIndex((s) => s.id === id) + 1
  const announcements: Announcements = {
    onDragStart: ({ active }) => `Picked up step ${position(active.id)}.`,
    onDragOver: ({ active, over }) =>
      over ? `Step ${position(active.id)} is over position ${position(over.id)}.` : undefined,
    onDragEnd: ({ active, over }) =>
      over ? `Step ${position(active.id)} moved to position ${position(over.id)}.` : 'Step dropped.',
    onDragCancel: ({ active }) => `Moving step ${position(active.id)} was cancelled.`,
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    onChange(arrayMove(steps, position(active.id) - 1, position(over.id) - 1))
  }

  const setText = (id: string, text: string) => onChange(steps.map((s) => (s.id === id ? { ...s, text } : s)))
  const remove = (id: string) => onChange(steps.filter((s) => s.id !== id))

  // Pasting several steps at once (e.g. a method copied from a website)
  // spreads them over separate steps instead of filling one.
  function onPaste(e: ClipboardEvent<HTMLTextAreaElement>, index: number) {
    const parts = splitSteps(e.clipboardData.getData('text'))
    if (!parts) return
    e.preventDefault()
    const { value, selectionStart, selectionEnd } = e.currentTarget
    const pasted = [...parts]
    pasted[0] = value.slice(0, selectionStart) + pasted[0]
    pasted[pasted.length - 1] += value.slice(selectionEnd)
    const [first, ...rest] = pasted
    onChange([
      ...steps.slice(0, index),
      { ...steps[index], text: first },
      ...rest.map((text) => newStep(text)),
      ...steps.slice(index + 1),
    ])
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={onDragEnd}
      accessibility={{ announcements }}
    >
      <SortableContext items={steps} strategy={verticalListSortingStrategy}>
        <ol className="step-fields">
          {steps.map((step, i) => (
            <SortableStep
              key={step.id}
              step={step}
              number={i + 1}
              canRemove={steps.length > 1}
              onText={(text) => setText(step.id, text)}
              onRemove={() => remove(step.id)}
              onPaste={(e) => onPaste(e, i)}
            />
          ))}
        </ol>
      </SortableContext>
    </DndContext>
  )
}

function SortableStep(props: {
  step: Step
  number: number
  canRemove: boolean
  onText: (text: string) => void
  onRemove: () => void
  onPaste: (e: ClipboardEvent<HTMLTextAreaElement>) => void
}) {
  const { step, number } = props
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } =
    useSortable({ id: step.id })

  return (
    <li
      ref={setNodeRef}
      className={isDragging ? 'dragging' : undefined}
      style={{ transform: CSS.Translate.toString(transform && { ...transform, x: 0 }), transition }}
    >
      <button
        type="button"
        ref={setActivatorNodeRef}
        className="drag-handle"
        aria-label={`Reorder step ${number}`}
        {...attributes}
        {...listeners}
      >
        ⠿
      </button>
      <textarea
        rows={3}
        aria-label={`Step ${number}`}
        value={step.text}
        onChange={(e) => props.onText(e.target.value)}
        onPaste={props.onPaste}
      />
      {props.canRemove && (
        <button type="button" className="icon-button" aria-label={`Remove step ${number}`} onClick={props.onRemove}>
          ×
        </button>
      )}
    </li>
  )
}
