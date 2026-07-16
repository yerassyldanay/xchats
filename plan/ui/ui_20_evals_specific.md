# Evaluation Details Page (`/evals/:launchId`)

## Goal

This page helps users answer one question:

> **Which prompt strategy performs best for each model, and why?**

The page should support a natural decision-making workflow:

1. Select an evaluation family (WhatsApp, File Parsing, etc.).
2. Compare prompt strategies across models.
3. Choose the most promising strategy.
4. Inspect the individual test cases that produced those results.
5. Understand exactly why something passed or failed.

The current page tries to show everything simultaneously. Instead, the UI should guide the user from summary → comparison → investigation.

---

# Page Layout

```
Header
│
├── Family Tabs
│
├── Decision Matrix
│
├── Active Filters
│
├── Test Cases List
│
└── Footer
```

Every section has a clear responsibility.

---

# 1. Header

Large launch title.

```
2026-07-14-15-53-39-2cbb
```

Secondary information

```
Started Jul 14, 2026 • Duration 24m 18s
```

Status badge

```
Completed
```

Right side

```
Export Report
More actions
```

Nothing else.

No statistics.

No KPIs.

---

# 2. Family Tabs

The first navigation level.

```
WhatsApp Responses (12)

File Parsing (4)
```

Changing the tab replaces everything below.

Only one family is visible at a time.

---

# 3. Decision Matrix

This is the primary content of the page.

Users should immediately understand:

> Which prompt strategy should I use?

The matrix occupies almost the full page width.

Rows

```
GPT-4o

GPT-4.1

Claude 3.5 Sonnet

Gemini 1.5 Pro
```

Columns

Prompt strategies.

Examples

```
V1 Control

V2 Improved

V3 Escalation

V4 Routed

V5 Experiment
```

Each strategy represents an implementation approach, not necessarily one physical prompt file.

For example:

```
V4 Routed

contains

kk frame

ru frame

↓

one strategy

↓

one column
```

---

## Matrix Cell

Each cell contains only the information required for comparison.

Large

```
94%
```

Below

```
17 / 18 tests
```

Below

```
$0.0132
```

Below

```
1.3 s
```

Nothing more.

Avoid squeezing additional badges or metadata inside the cell.

---

## Metric Toggle

Top-right of matrix

```
Pass Rate

Cost

Latency
```

Changing the metric changes the visual emphasis.

Example

Pass Rate

Large percentage

Cost/latency smaller.

Latency

Large latency

Pass rate secondary.

---

# Prompt Strategy Interaction

Prompt names are intentionally short inside the matrix.

Examples

```
V4 Routed

V5 Exp.
```

The full prompt should never be permanently visible inside the table.

Instead, clicking the prompt header opens a popover or side sheet containing:

```
Strategy Name

Description

Languages

Frames included

Number of test questions

Experiment name

Notes

Button

View full prompt
```

Clicking **View full prompt** opens a dedicated prompt viewer (modal or drawer) displaying the complete prompt with syntax highlighting, preserving formatting and allowing easy copy.

This keeps the matrix compact while making every strategy fully inspectable.

---

# Multiple Experiments

If prompt strategies belong to different experiments, never mix them.

Instead:

```
Experiment

General Conversation

[matrix]
```

```
Experiment

Escalation

[matrix]
```

```
Experiment

Language Routing

[matrix]
```

Separate cards.

Separate titles.

---

# Different Test Sets

If one strategy used another question set, never average it.

Display another matrix.

Show warning banner.

Example

```
⚠

This strategy was evaluated using a different test set.

Results cannot be directly compared with the table above.
```

---

# Matrix Selection

Clicking any matrix cell activates it.

Example

Selected

```
GPT-4o

×

V4 Routed
```

The selected cell receives

* border
* subtle background
* checkmark indicator

Everything below automatically filters.

---

# 4. Filters

Compact toolbar.

```
Model ▼

Prompt Strategy ▼

Status ▼

☑ Failures only
```

Below filters show active chips.

Example

```
GPT-4o

V4 Routed

Failures only
```

Each chip removable.

---

# 5. Test Cases

This section explains the matrix.

Title

```
Individual Test Results

18 questions
```

Toolbar

```
Collapse all

Expand all

Sort ▼
```

---

## Test Case Card

One card per evaluation question.

Header

```
Q1

Greeting

PASS
```

or

```
FAIL
```

Small secondary line

```
Customer message preview
```

Collapsed by default.

Users only expand the questions they care about.

---

# Expanded Question

The expanded state contains a table instead of stacked cards.

Columns

| Model | Status | Raw Reply | After Injection | Cost | Latency | Checks |
| ----- | ------ | --------- | --------------- | ---- | ------- | ------ |

One row per model.

This makes comparison dramatically easier than repeating four cards.

---

## Customer Message

Shown once.

Not repeated for every model.

Displayed above the table.

Example

```
Customer

Сәлем!

Қалай көмектесе аласыз?
```

Optional

```
Conversation history

Expand
```

---

## Raw Reply

Model output before processing.

Scrollable.

Monospace.

Copy button.

---

## After Injection

Final customer-facing response.

Highlighted.

Green background if modified.

---

## Cost

Simple.

```
$0.00052
```

---

## Latency

```
1.2 s
```

---

## Checks

Collapsed by default.

Click

```
5 / 5

▼
```

expands

```
PASS

Greeting

PASS

Language

PASS

Policy

PASS

Formatting

PASS

Grounding
```

Button

```
View raw JSON
```

opens a modal with the complete evaluation payload instead of expanding the page vertically.

---

# Navigation Behavior

Users should naturally move through the page like this:

```
Choose family

↓

Compare strategies

↓

Select best cell

↓

Automatically filter results

↓

Inspect failed questions

↓

Open raw evaluation only when needed
```

The matrix remains the primary focus, while the detailed results act as supporting evidence rather than competing with it. This creates a clear information hierarchy and avoids the "wall of cards" effect present in the current design.
