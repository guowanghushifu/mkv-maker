# Track Editor Switches Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the track editor's `Include` and `Default` checkbox visuals with switch controls while preserving all existing selection and exclusive-default behavior.

**Architecture:** Add one small reusable `Switch` component in the shared frontend component layer, style it with a dedicated CSS file, and keep the track business logic in `web/src/features/draft/trackTable.ts` unchanged. Then swap the track editor table cells over to the new component, update the editor tests from checkbox queries to switch queries, and run targeted frontend regression plus a build to confirm the visual-only change did not alter workflow behavior.

**Tech Stack:** React 19, TypeScript, CSS, Vitest, Testing Library

---

## File Map

- Create: `web/src/components/Switch.tsx`
  Responsibility: render a compact reusable switch control with `role="switch"`, checked state, disabled state, and a minimal `onChange` callback surface
- Create: `web/src/styles/switch.css`
  Responsibility: define the shared switch visuals, focus treatment, disabled treatment, and on/off thumb motion
- Create: `web/src/test/Switch.test.tsx`
  Responsibility: verify the shared switch component exposes switch semantics and respects disabled state
- Modify: `web/src/styles/index.css`
  Responsibility: import the new shared switch stylesheet once for the frontend
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
  Responsibility: replace the `Include` and `Default` checkbox cells with the shared `Switch` component while preserving the current selection/default helpers
- Modify: `web/src/styles/workflow-pages.css`
  Responsibility: give the editor table enough room for the switch controls and center them cleanly in the table cells
- Modify: `web/src/test/TrackEditorPage.test.tsx`
  Responsibility: verify the track editor exposes switches, keeps current default-clearing behavior, and keeps disabled default controls non-interactive

### Task 1: Add The Shared Switch Component

**Files:**
- Create: `web/src/components/Switch.tsx`
- Create: `web/src/styles/switch.css`
- Create: `web/src/test/Switch.test.tsx`
- Modify: `web/src/styles/index.css`

- [ ] **Step 1: Write the failing switch component tests**

```tsx
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Switch } from '../components/Switch';

describe('Switch', () => {
  it('renders a switch with the current checked state', () => {
    render(<Switch aria-label="Include English Atmos" checked onChange={vi.fn()} />);

    expect(screen.getByRole('switch', { name: /include english atmos/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('invokes onChange when activated and ignores clicks while disabled', () => {
    const onChange = vi.fn();
    const { rerender } = render(<Switch aria-label="Default Commentary" checked={false} onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);

    rerender(<Switch aria-label="Default Commentary" checked={false} disabled onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('switch', { name: /default commentary/i })).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the switch tests to verify they fail**

Run: `npm test -- --run src/test/Switch.test.tsx`

Expected: FAIL because `web/src/components/Switch.tsx` does not exist yet.

- [ ] **Step 3: Write the minimal reusable switch and shared styles**

```tsx
import type { KeyboardEventHandler } from 'react';

type SwitchProps = {
  checked: boolean;
  disabled?: boolean;
  className?: string;
  'aria-label': string;
  onChange?: () => void;
};

export function Switch({
  checked,
  disabled = false,
  className = '',
  'aria-label': ariaLabel,
  onChange,
}: SwitchProps) {
  const classes = ['ui-switch', checked ? 'is-on' : 'is-off', className].filter(Boolean).join(' ');

  const handleKeyDown: KeyboardEventHandler<HTMLButtonElement> = (event) => {
    if (disabled) {
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      onChange?.();
    }
  };

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      className={classes}
      onClick={() => onChange?.()}
      onKeyDown={handleKeyDown}
    >
      <span className="ui-switch-track" aria-hidden="true">
        <span className="ui-switch-thumb" />
      </span>
    </button>
  );
}
```

```css
.ui-switch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.7rem;
  height: 1.6rem;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.ui-switch:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.ui-switch:focus-visible {
  outline: 2px solid var(--accent-strong);
  outline-offset: 2px;
  border-radius: 999px;
}

.ui-switch-track {
  position: relative;
  width: 2.5rem;
  height: 1.4rem;
  border-radius: 999px;
  background: rgba(145, 160, 179, 0.45);
  box-shadow: inset 0 0 0 1px rgba(111, 125, 147, 0.18);
  transition: background 140ms ease;
}

.ui-switch.is-on .ui-switch-track {
  background: rgba(74, 144, 245, 0.9);
}

.ui-switch-thumb {
  position: absolute;
  top: 0.15rem;
  left: 0.15rem;
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 2px 8px rgba(24, 33, 50, 0.18);
  transition: transform 140ms ease;
}

.ui-switch.is-on .ui-switch-thumb {
  transform: translateX(1.1rem);
}
```

```css
@import './tokens.css';
@import './app-shell.css';
@import './switch.css';
@import './workflow-pages.css';
```

- [ ] **Step 4: Run the switch tests to verify they pass**

Run: `npm test -- --run src/test/Switch.test.tsx`

Expected: PASS, confirming `role="switch"`, `aria-checked`, and disabled handling.

- [ ] **Step 5: Commit the shared switch component**

```bash
git add web/src/components/Switch.tsx web/src/styles/switch.css web/src/styles/index.css web/src/test/Switch.test.tsx
git commit -m "feat: add shared switch control"
```

### Task 2: Replace Track Editor Checkboxes With Switches

**Files:**
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
- Modify: `web/src/styles/workflow-pages.css`
- Modify: `web/src/test/TrackEditorPage.test.tsx`

- [ ] **Step 1: Write the failing track editor tests for switch semantics**

```tsx
it('renders include and default controls as switches', () => {
  render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

  expect(screen.getByRole('switch', { name: /include english atmos/i })).toBeInTheDocument();
  expect(screen.getByRole('switch', { name: /default commentary/i })).toBeInTheDocument();
  expect(screen.queryByRole('checkbox', { name: /include english atmos/i })).not.toBeInTheDocument();
});

it('clears the previous default audio track when a new default switch is turned on', () => {
  const onChange = vi.fn();
  render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

  fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));

  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({
      audio: [
        expect.objectContaining({ id: 'a1', default: false }),
        expect.objectContaining({ id: 'a2', default: true }),
      ],
    }),
  );
});

it('keeps a default switch disabled when the track is not included', () => {
  const onChange = vi.fn();
  render(
    <TrackEditorPage
      locale="en"
      draft={{
        ...createDraft(),
        subtitles: [
          { id: 's1', name: 'English PGS', language: 'eng', selected: false, default: false },
          { id: 's2', name: 'Signs', language: 'eng', selected: true, default: true },
        ],
      }}
      onChange={onChange}
    />,
  );

  const disabledDefault = screen.getByRole('switch', { name: /default english pgs/i });
  expect(disabledDefault).toBeDisabled();

  fireEvent.click(disabledDefault);
  expect(onChange).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: Run the track editor tests to verify they fail**

Run: `npm test -- --run src/test/TrackEditorPage.test.tsx`

Expected: FAIL because the page still renders `checkbox` controls instead of `switch` controls.

- [ ] **Step 3: Swap the table cells to the shared switch component and adjust table spacing**

```tsx
import { Button } from '../../components/Button';
import { Switch } from '../../components/Switch';
import { getMessages, type Locale } from '../../i18n';
import { moveTrackRow, setExclusiveDefault, toggleTrackSelected } from './trackTable';

// ...

<td className="track-toggle-cell">
  <Switch
    aria-label={text.editor.include(track.name)}
    checked={track.selected}
    onChange={() => {
      if (group === 'audio') {
        updateAudio(toggleTrackSelected(draft.audio, track.id));
        return;
      }
      updateSubtitles(toggleTrackSelected(draft.subtitles, track.id));
    }}
  />
</td>
<td className="track-toggle-cell">
  <Switch
    aria-label={text.editor.default(track.name)}
    checked={track.default}
    disabled={!track.selected}
    onChange={() => {
      if (group === 'audio') {
        updateAudio(setExclusiveDefault(draft.audio, track.id));
        return;
      }
      updateSubtitles(setExclusiveDefault(draft.subtitles, track.id));
    }}
  />
</td>
```

```css
.track-editor-table .col-include,
.track-editor-table .col-default {
  width: 5.4rem;
}

.track-editor-table .track-toggle-cell {
  text-align: center;
}

.track-editor-table .track-toggle-cell .ui-switch {
  margin: 0 auto;
}
```

- [ ] **Step 4: Run the updated editor tests and the shared switch tests**

Run: `npm test -- --run src/test/Switch.test.tsx src/test/TrackEditorPage.test.tsx`

Expected: PASS, confirming the editor now exposes switches while preserving the existing include/default behavior.

- [ ] **Step 5: Commit the track editor integration**

```bash
git add web/src/features/draft/TrackEditorPage.tsx web/src/styles/workflow-pages.css web/src/test/TrackEditorPage.test.tsx
git commit -m "feat: use switches in track editor"
```

### Task 3: Run Frontend Regression For The Visual Control Swap

**Files:**
- Test: `web/src/test/Switch.test.tsx`
- Test: `web/src/test/TrackEditorPage.test.tsx`
- Test: `web/src/test/trackTable.test.ts`

- [ ] **Step 1: Run the focused editor and helper regression suite**

Run: `npm test -- --run src/test/Switch.test.tsx src/test/TrackEditorPage.test.tsx src/test/trackTable.test.ts`

Expected: PASS, confirming the new UI control did not change track-selection helper behavior.

- [ ] **Step 2: Run the frontend build**

Run: `npm run build`

Expected: PASS, producing a Vite build without TypeScript or bundling errors.

- [ ] **Step 3: Confirm the working tree is clean after the task commits**

Run: `git status --short`

Expected: no output.

## Self-Review

- Spec coverage: the plan covers the shared switch component, track editor integration for both `Include` and `Default`, preserved disabled/default behavior, accessibility semantics via `role="switch"`, and the requested frontend test updates.
- Placeholder scan: no `TODO`, `TBD`, or deferred implementation language remains in the tasks.
- Type consistency: the plan uses one shared `Switch` API (`checked`, `disabled`, `aria-label`, `onChange`) consistently across the component task and the track editor integration task.
