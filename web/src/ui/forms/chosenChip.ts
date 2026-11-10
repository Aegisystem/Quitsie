import { useLayoutEffect, useState } from 'react'

// A row of chips that scrolls sideways (.chips-scroll) keeps the chosen one in
// sight: when the row opens with it out of view, as the fourth split method is
// on a phone, and when it changes without a tap, as a suggested Category does.
// Only the row moves, never the page, and a chip already in sight stays put, so
// the Usuario can still scroll the row to read the other choices.
export function useChosenChipInSight(chosen: string | undefined): (row: HTMLElement | null) => void {
  // State, not a ref: the row mounts again each time its screen comes back.
  const [row, setRow] = useState<HTMLElement | null>(null)
  useLayoutEffect(() => {
    const chip = row?.querySelector('input:checked')?.closest('.chip')
    if (!row || !chip) return
    const within = row.getBoundingClientRect()
    const box = chip.getBoundingClientRect()
    if (box.left >= within.left && box.right <= within.right) return
    row.scrollLeft += box.left - within.left - (within.width - box.width) / 2
  }, [row, chosen])
  return setRow
}
