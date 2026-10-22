import { describe, expect, test } from 'vitest'
import { splitSteps } from './steps'

describe('splitSteps', () => {
  test('splits at blank lines, keeping line breaks within a step', () => {
    expect(splitSteps('Make the sauce:\nstir well.\n\nBoil the pasta.\n\n\nServe.')).toEqual([
      'Make the sauce:\nstir well.',
      'Boil the pasta.',
      'Serve.',
    ])
  })

  test('splits numbered lines and removes the numbering', () => {
    expect(splitSteps('1. Chop.\n2) Fry.\nStep 3: Serve.')).toEqual(['Chop.', 'Fry.', 'Serve.'])
  })

  test('keeps wrapped lines with their numbered step', () => {
    expect(splitSteps('1. Chop the onions\nfinely.\n2. Fry.')).toEqual(['Chop the onions\nfinely.', 'Fry.'])
  })

  test('removes numbering from blank-line separated steps too', () => {
    expect(splitSteps('1. Chop.\n\n2. Fry.')).toEqual(['Chop.', 'Fry.'])
  })

  test('handles Windows line endings', () => {
    expect(splitSteps('Chop.\r\n\r\nFry.')).toEqual(['Chop.', 'Fry.'])
  })

  test.each([
    ['a single line', 'Whisk everything together.'],
    ['a single paragraph', 'Whisk everything\ntogether.'],
    ['one numbered line', '1. Chop.'],
    ['lines starting with quantities', '200 g flour\n2 eggs'],
    ['only blank parts', '\n\n  \n\n'],
  ])('leaves %s alone', (_, text) => {
    expect(splitSteps(text)).toBeNull()
  })
})
