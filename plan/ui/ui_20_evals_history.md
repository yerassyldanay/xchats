## Canvas: `/evals` — Evaluation Launches

### Goal

This page is **not an analytics dashboard**.

Its only purpose is to help users find and open a specific evaluation launch.

The page should communicate one simple message:

> **These are automated evaluation tests we run on your AI assistant. Open any launch to inspect the quality of responses, prompts, and every individual check.**

Users shouldn't have to interpret charts or metrics before opening a launch.

---

# Header

**Title**

> Evaluation Tests

**Description**

> Every launch contains automated quality tests performed by our evaluation framework.
>
> Open any launch to review prompt performance, compare models, inspect every response, and understand exactly why a test passed or failed.

---

# Top Bar

Left:

**Total launches**

```
128 evaluation launches
```

Nothing else.

No average pass rate.

No total checks.

No global latency.

No costs.

Those numbers are meaningless before choosing a launch.

Right:

```
Search launch...

[ Family ▼ ]

[ Model ▼ ]

[ Status ▼ ]
```

---

# Main Content

A clean table.

Each row = one evaluation launch.

Columns:

| Launch | Test Families | Models | Result | Started | Duration |
| ------ | ------------- | ------ | ------ | ------- | -------- |

---

## Launch column

Large launch id

```
2026-07-14-15-53-39-2cbb
```

Small secondary text

```
Started Jul 14, 2026
15:53 UTC
```

Whole row is clickable.

---

## Test Families

Small colored pills

```
WhatsApp Responses

File Parsing
```

This immediately tells users what was evaluated.

---

## Models

Display only first 2–3 models.

Example

```
GPT-4o

Claude 3.5 Sonnet

+2 more
```

Avoid showing four long badges stretching across the page.

---

## Result

Large text

```
14 / 16
```

Secondary

```
88% passed
```

Small progress bar underneath.

Green

Yellow

Red

depending on pass rate.

This is the primary information users care about.

---

## Started

```
14 Jul

15:53
```

Simple.

---

## Duration

```
24m
```

---

# Row interaction

Hover highlights entire row.

Arrow on right.

Click anywhere opens

```
/evals/{launch}
```

No explicit "Open" button required.

---

# Empty State

Illustration of a flask/checkmark.

Title

> No evaluation launches yet

Description

> Run your first evaluation to verify the quality of prompts, model responses, and safety checks.

Button

```
Run Evaluation
```

(if applicable)

---

# Why this is better

The current page mixes together:

* launch identification
* families
* models
* statistics
* badges

without any visual hierarchy.

The redesigned page makes scanning effortless:

1. Find the launch.
2. See what was tested.
3. See which models participated.
4. Check whether it passed.
5. Open it.

Everything else belongs inside the launch details page.

---

# Visual Style

* Plenty of white space between rows.
* Card-like table rows with 12–16px radius.
* Soft borders instead of heavy outlines.
* Family badges use accent colors (purple, green).
* Model badges use neutral gray.
* Result uses semantic colors (green, amber, red).
* Entire row behaves like a clickable card.
* Avoid dashboard widgets and KPI cards—the page is a directory, not a reporting dashboard.
