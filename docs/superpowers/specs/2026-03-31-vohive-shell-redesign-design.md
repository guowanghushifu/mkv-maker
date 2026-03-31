# VoHive Shell Redesign Design

## Goal

Refit the `web/` frontend so it visually and structurally resembles the referenced VoHive admin site, while preserving the existing mkv-maker workflow logic and step order.

The redesign should move the app from the current dark, hero-style workflow shell to a light admin-console shell with:

- a standalone login page
- a persistent left sidebar after login
- a compact top bar
- white card surfaces on a light background
- page-local content regions instead of one large global workflow header

## Non-Goals

- Do not change the underlying workflow state machine or step order.
- Do not introduce new business pages or backend APIs.
- Do not split the current workflow into independent routed modules with new persistence rules.
- Do not clone the reference site literally; the target is strong visual and structural alignment, not a pixel-perfect copy.

## Approved Design

### Information Architecture

- Keep `login` as a standalone entry screen outside the admin shell.
- After successful login, render all workflow pages inside a single shared admin shell.
- Replace the current top hero plus horizontal step list with a left sidebar containing four primary items:
  - `源扫描`
  - `BDInfo 解析`
  - `轨道编辑`
  - `复核提交`
- Use the current workflow step to drive the active sidebar item. No additional navigation model is needed.
- Move the current global workflow context out of the page header and into page-level summary regions:
  - a top row of small status/summary cards
  - a right-side context panel for current source, playlist, output, and task information

### Shell Layout

- `Layout` becomes the shared post-login shell.
- The shell should be a two-column structure:
  - fixed-width left sidebar
  - flexible main region
- The main region should contain:
  - a compact top bar with current page title, subtitle, and small utility actions
  - a page body area with card-based content sections
- The sidebar should include:
  - product brand block
  - four workflow navigation items
  - a small operator/session card near the bottom

### Page Mapping

#### Login Page

- Render as a separate light login screen.
- Use a centered white card, soft gray background, blue-violet brand mark, and gradient primary action button inspired by the reference site.
- Do not show the sidebar or top bar on this page.

#### Scan Page

- Keep the existing scan actions and selection behavior.
- Reframe the page as an admin-style source workspace:
  - page title and actions at the top
  - compact summary cards underneath
  - main source list area in a large white card
  - right-side context and workflow-progress cards
- Existing source cards may be restyled or converted into a denser list/table-like presentation if that better fits the shell.

#### BDInfo Page

- Keep the current textarea-driven parsing flow.
- Use a split main layout:
  - composer/input region
  - supporting context and parsed-summary region
- Keep the sample BDInfo content, but present it as a secondary support card rather than a raw utility block.

#### Track Editor Page

- Keep the current editing workflow and data behavior.
- Present the track table and filename editing area as the most table-centric page in the new shell.
- Align row density, header treatment, inputs, and grouping cards with the light admin style.

#### Review Page

- Keep the current review, submit, progress, and log behavior.
- Reframe the page into:
  - final track/output summary card(s)
  - primary action area
  - job status and progress card
  - log console card
- The log area can remain visually distinct, but it should still sit inside the light shell rather than the old dark industrial theme.

### Visual System

- Replace the current dark palette with a light product UI palette:
  - light gray app background
  - white primary surfaces
  - pale blue borders and selected states
  - bright blue primary action color
  - blue-violet brand accent for login/brand elements
  - restrained green and red for success and error states
- Increase consistency around large-radius surfaces:
  - main cards around `20px` to `24px`
  - buttons and inputs around `12px` to `16px`
  - pills and badges fully rounded where appropriate
- Reduce visual weight:
  - lighter shadows
  - subtler borders
  - less saturated error and warning styling
- Shift typography toward a clean admin-console hierarchy:
  - large bold page titles
  - subdued subtitles and labels
  - consistent medium-weight table and form text

### Component and Styling Strategy

- Concentrate most changes in layout and styling layers.
- `web/src/components/Layout.tsx` should be restructured substantially to support the new shell.
- `web/src/styles/tokens.css` should be updated to define the new light theme tokens.
- `web/src/styles/app-shell.css` should be rewritten around the sidebar, top bar, summary cards, and new page container structure.
- `web/src/styles/workflow-pages.css` should be updated for:
  - login screen
  - buttons
  - forms
  - white card surfaces
  - tables/lists
  - empty states
  - progress and log panels
- Page components should only change where necessary to fit the new structure and class naming. Business logic should stay intact.

### Interaction Rules

- Sidebar items reflect the current workflow step and may also be used to request step navigation, but existing guardrails remain authoritative.
- Top-bar content should switch based on the current step.
- Context cards are read-only summaries, not editing surfaces.
- The shell should remain usable on narrower widths, but the primary visual target is desktop admin-console layout.

## Scope

- Redesign the post-login application shell.
- Redesign the login page to match the new light visual direction.
- Update page structure and styles for all workflow steps so they fit the new shell.
- Update frontend tests that depend on the old shell markup where necessary.

## Risks And Constraints

- The existing tests may be tightly coupled to the old DOM structure.
- The review/log page can become visually inconsistent if the console treatment is not intentionally integrated into the new light shell.
- Over-copying the reference site would increase effort without helping the mkv-maker workflow, so visual borrowing should stay focused on shell and system language.

## Verification Strategy

- Run the frontend test suite after the redesign.
- Verify login, step transitions, source selection, BDInfo submission, track editing, and review-job rendering.
- Manually inspect layout behavior for desktop and narrow-width states.
