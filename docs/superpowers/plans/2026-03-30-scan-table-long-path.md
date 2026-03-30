# Scan Table Long Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the scan results table stable for very long source paths by using truncation with hoverable full-path disclosure.

**Architecture:** The fix stays local to the existing scan feature. A small render change in the scan page adds a dedicated path text element, and CSS makes the full table denser while locking the table into a fixed layout that respects the container width.

**Tech Stack:** React, TypeScript, Vitest, Testing Library, CSS

---

### Task 1: Add a long-path regression test

**Files:**
- Create: `web/src/test/ScanPage.test.tsx`
- Test: `web/src/test/ScanPage.test.tsx`

- [ ] **Step 1: Write the failing test**

```tsx
it('renders long source paths with hoverable full text metadata', () => {
  render(
    <ScanPage
      loading={false}
      error={null}
      sources={[longPathSource]}
      selectedSourceId={null}
      onScan={() => {}}
      onSelectSource={() => {}}
      onNext={() => {}}
    />,
  );

  const pathText = screen.getByText(longPathSource.path);
  expect(pathText).toHaveAttribute('title', longPathSource.path);
  expect(pathText).toHaveClass('source-path-text');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- ScanPage.test.tsx --runInBand`
Expected: FAIL because the rendered path text does not yet have the `title` attribute or truncation class.

- [ ] **Step 3: Write minimal implementation**

```tsx
<td className="source-path-cell">
  <span className="source-path-text" title={source.path}>
    {source.path}
  </span>
</td>
```

```css
.source-path-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- ScanPage.test.tsx --runInBand`
Expected: PASS

### Task 2: Make the scan table denser and width-safe

**Files:**
- Modify: `web/src/features/sources/ScanPage.tsx`
- Modify: `web/src/styles/app.css`
- Test: `web/src/test/ScanPage.test.tsx`

- [ ] **Step 1: Write the failing styling-oriented assertion**

```tsx
const table = container.querySelector('.source-table');
expect(table).toBeInTheDocument();
```

Add CSS hooks for fixed layout and path truncation support in implementation.

- [ ] **Step 2: Run the targeted test suite**

Run: `npm test -- ScanPage.test.tsx App.test.tsx --runInBand`
Expected: existing scan flow tests pass, new regression is still the only failing assertion before implementation.

- [ ] **Step 3: Write minimal implementation**

```css
.source-table {
  width: 100%;
  max-width: 100%;
  table-layout: fixed;
  min-width: 0;
  font-size: 0.88rem;
}

.source-table th,
.source-table td {
  font-size: 0.88rem;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- ScanPage.test.tsx App.test.tsx --runInBand`
Expected: PASS with scan-page regression and existing app flow tests green.
