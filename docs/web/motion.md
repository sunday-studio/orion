# Motion guidelines

Distilled from three references: [Details that make interfaces feel better](https://interfaces.dev/magazine/issues/details-that-make-interfaces-feel-better),
[How I use shared layout animations](https://interfaces.dev/magazine/issues/how-i-use-shared-layout-animations) and the [Interfaces cheat sheet](https://interfaces.dev/cheat-sheet).
The goal is calm motion: things that persist move to their new place, nothing flashes in.

## Rules we follow

1. **Move, don't swap.** An element that exists before and after a change glides to its new position
   (shared layout), using [Motion](https://motion.dev/docs/react-layout-animations) with a no-bounce
   spring (`layoutSpring`: duration 0.45, bounce 0). `Segmented` and the header navigation share one
   `layoutId` thumb, `AnimatedHeight` springs dialogs between sizes, and `Tabs` slides its underline.
   Table rows do not move up and down: rows that are new in a changed list fade in (opacity only).
2. **Nothing animates on first load.** Pages, tab panels and tables render in place. Motion only plays
   when something changes.
3. **Transitions for interactions, keyframes for one-shot sequences.** Transitions retarget when the
   user changes their mind mid-way, keyframes do not. Layout motion uses the Web Animations API for the
   same reason.
4. **Calm timing, no bounce.** Layout moves use the spring above (about 450 ms) or 380 ms on `cubic-bezier(0.32, 0.72, 0, 1)` for CSS.
   Overlays enter in about 260 ms on `cubic-bezier(0.22, 1, 0.36, 1)`. Frequent interactions (hover
   colour) use 150 ms or less. Tokens live in `design-system/motion.css`.
5. **Exits are subtler than entrances.** Dialogs scale from 0.96 on enter and only to 0.98 on exit,
   with opacity doing most of the work.
6. **Name the properties.** Use `transition-[left,width]`, `transition-colors` and so on. Never
   `transition-all`.
7. **Buttons press.** `active:scale-[0.98]` with a 200 ms scale transition.
8. **Theme switches are instant.** Transitions are suspended for a frame when light and dark swap.
9. **Reduced motion.** `prefers-reduced-motion: reduce` collapses every animation and transition.
   Hover styles stay inside Tailwind's `hover:` (which only applies where hover exists).
10. **Typography.** Headings use `text-wrap: balance`, paragraphs `text-wrap: pretty`, numbers in tables
    use tabular figures.

## Building blocks

| Piece | Use |
|---|---|
| `Segmented`, header nav | sliding thumb via Motion `layoutId` |
| `TabsList` | sliding underline (Base UI indicator, CSS) |
| `AnimatedHeight` | wrap form content that changes size, for example the create monitor dialog |
| `useFlip` (used by `GridTable` and `DataTable`) | give rows `flipId` (or `data-flip-id`) so new rows fade in when the list changes |
| `ds-fetching` / `ds-settled` | dim a table while a refetch is in flight, keep rows with `placeholderData: keepPreviousData` |

## Not done yet

- Exit animations for removed table rows (they simply disappear).
- Icon swap blur and scale (copy to check mark and similar).
