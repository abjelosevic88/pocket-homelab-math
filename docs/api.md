# API

Everything the web app does goes through a JSON API at `/api/v1`. Sign in with a session cookie, or send a personal token (Settings → API & shortcuts) as `Authorization: Bearer pmth_…`. Every person sees only their own notebooks, calculations, answers and practice — admins included.

## Working things out

| | |
|---|---|
| `GET /calculate?q=…` · `POST /calculate {"expr": "…", "angle": "deg"\|"rad", "units": true}` | Works out one or more lines. `result` is the last line: `{"kind": "value", "text": "195.58 BAM", "number": 195.58, "unit": "BAM"}`; `lines` has every line. |
| `GET /convert?value=5&from=km&to=mi` | A unit or money conversion. |
| `GET /reference` | Converter kinds and units, every function, currencies and the exchange rates in use. |
| `GET /rates` · `POST /rates` | Exchange rates per euro (European Central Bank, fetched twice a day; currencies tied to the euro always). POST fetches now. |

The calculator reads `+ - * / ^ !`, brackets, `2x`, units (`5 km + 300 m`, `60 mph to km/h`, `6.5 L/100km to mpg`), money (`100 EUR in BAM`, `$20`), percentages (`20% of 80`, `50 + 10%`, `10% off 80`), names (`x = 5`), functions (`f(x) = x^2`, `f'(3)`), labels (`Rent: 450 EUR`), `total`, `ans`, dates (`2026-12-25 - today`), equations (`2x + 3 = 11`, `solve(…)`), `integral`, `deriv`, `sum`, statistics, `pmt`/`fv`/`pv`/`compound`, and `to hex|bin|oct|sci|fraction|%`. The same rules run in the browser (`web/src/lib/calc.ts`) and on the server (`internal/calc`), both checked against `internal/calc/testdata/vectors.json`.

## Notebooks and cells

| | |
|---|---|
| `GET /notebooks?q=&subject=&archived=1` | Your notebooks (cell count, kinds, a first line) and your folders. |
| `POST /notebooks` | `{"title": "Trip", "subject": "Travel", "template": "trip", "cells": [{"kind": "calc", "body": "2 * 320 km"}], "source_ref": "…"}` — templates: blank, budget, trip, renovation, homework, graph, geometry, data, loan. `source_ref` makes it idempotent for another app. |
| `GET /notebooks/ID` | The notebook and its cells; calculation cells carry `results` (one per line), worked out now. |
| `PUT /notebooks/ID` | title, subject, about, color, angle, units, tags, pinned, archived. |
| `DELETE /notebooks/ID` · `POST /notebooks/ID/copy` | |
| `PUT /notebooks/ID/share {"shared": true, "new": false}` | A read-only link `/s/KEY` (`new`: a new key, the old one stops working). |
| `GET /notebooks/ID/export.md` | The notebook as Markdown, sums with their answers. |
| `POST /notebooks/ID/cells` | `{"kind": "calc"\|"text"\|"graph"\|"geometry"\|"data"\|"solve", "body": "…", "data": {…}, "after": CELL_ID}` |
| `PUT /cells/ID` · `DELETE /cells/ID` · `POST /cells/ID/copy` · `PUT /notebooks/ID/order {"ids": […]}` | Each answers with every calculation's results and the notebook's new revision. |

## Ready calculations and kept answers

| | |
|---|---|
| `GET /formulas?q=` | `mine` (your own), `library` (the built-ins) and `categories`. |
| `GET /library/KEY` · `POST /library/KEY/run` · `POST /library/KEY/copy` | A built-in: paint, floor, wallpaper, skirting, concrete, power_cost, heating_area, loan, savings, compound, change, vat, discount, tip, unit_price, hourly, trip_fuel, consumption, travel_time, ev_charge, bmi, calories, pace, heart_zones, water, recipe_scale, oven, baking_pan, days_until, age, add_days, pythagoras, circle, cylinder, sphere, heron, quadratic, slope, ohm, kinetic, fall, density. |
| `POST /formulas` · `GET/PUT/DELETE /formulas/ID` · `POST /formulas/ID/run` | Your own: `name, about, category, icon, code, inputs[{name, label, unit, default, kind, options, from, help}], outputs[{label, expr, unit, decimals, main}]`. |
| run body | `{"values": {"wall_area": "32"}, "keep": true, "title": "Bedroom", "note": ""}` → `{"result": {"outputs": […], "lines": […]}, "run": {…}}` |
| `POST /formulas/try` | Work out a calculation that is not saved yet. |
| `POST /runs` | `{"key": "paint", "values": {…}, "title": "…"}` or `{"formula_id": 3, …}` — work it out and keep the answer (for other apps). |
| `GET /runs?formula_id=&key=` · `PUT /runs/ID` · `DELETE /runs/ID` · `GET /export.csv` | Kept answers. |

## Practice

| | |
|---|---|
| `GET /practice?learner=` | Rounds, and per person (you and each child): today, goal, streak, the last 30 days and accuracy per skill. |
| `POST /practice` | `{"learner": "Mila", "skill": "tables", "level": 2, "questions": 10, "correct": 9, "seconds": 64, "details": […]}` → `goal_met` when the day's goal was reached (and your Pocket Habits habit is ticked). |
| `GET /practice/ID` · `DELETE /practice/ID` | |

## For the rest of the box

| | |
|---|---|
| `GET /summary` | Counts, the last notebook and today's practice — for Homepage and Pocket AI. |
| `GET /day/YYYY-MM-DD` | `{"items": [{"time": "HH:MM", "kind": "notebook"\|"answer"\|"practice", "emoji", "text", "url"}]}` — for Pocket Day. |
| `GET /search?q=` | `{"results": [{"kind", "id", "title", "subtitle", "line", "url", "path"}]}` — notebooks, calculations and kept answers, for Pocket Search. |
| `GET /home` · `GET /templates` | The start page. |
| `GET /public/s/KEY` | A shared notebook, no account needed. |

## Pocket Core app links

Math offers two scopes to other Pocket apps (switched on per person in Pocket Core → Links):

- `math:read` — notebooks, calculations, kept answers, practice, search, day, and the exact calculator (`/calculate`, `/convert`, working out a calculation without keeping it).
- `math:write` — start a notebook, add cells, save a calculation, keep an answer (`POST /runs`). An app may change or remove only what it made itself; it can never share or delete your own notebooks.

Events (for Pocket Flows): `math.notebook_created`, `math.answer_kept`, `math.formula_saved`, `math.practised`, `math.shared`.

## For Pocket Search, Day and Hub

Math tells Pocket Core what it can do (the capabilities in its manifest, category `Tools`), and answers on one route each, in the shape every Pocket app uses. A person can call them with their own session or token; another app needs the route's own scope (`math:read` and `math:write` do not open them). Everything is the person's own.

| | Scope | |
|---|---|---|
| `GET /pocket/search?q=&limit=` | `pocket:search` | What `GET /search` finds, as `{"items": [{"id": "notebook:3" \| "formula:5" \| "formula:paint" \| "answer:7" \| "sum", "title", "snippet", "kind": "notebook\|formula\|answer\|sum", "date", "url", "score"}]}`. A typed sum comes first, worked out (`"15% of 240 = 36"`); then what has the words in its name, then what has them inside (a notebook's `snippet` is the line with its answer); your own things before the ready calculations. A search without a letter or a digit finds nothing |
| `GET /pocket/day/YYYY-MM-DD?tz=` | `pocket:day` | The rows of `GET /day`: `{"date", "summary": "1 notebook, 2 answers kept, 1 practice round", "items": [{"id": "notebook:3" \| "answer:7" \| "practice:9", "time", "title", "sub", "kind": "notebook\|answer\|practice", "note", "url"}], "metrics": [{"label": "Practice", "value": "19 of 20 right"}]}`. The day runs from midnight to midnight in `tz` (the box's time zone without it). A day with nothing has no summary and no items |
| `GET /pocket/widget` | `pocket:widget` | The notebooks of the start page, pinned first, then the ones worked on last, at most eight: `{"line", "items": [{"id": "notebook:3", "title", "detail": "School · yesterday", "url": "/n/3"}], "url": "/notebooks"}`. `line` is there only when somebody practised today: "12 of 20 practice questions today", or "Practice today: you 12 of 20, Mila 20 of 20" |
| `POST /pocket/capture` | `pocket:capture` | `{"request_id", "kind": "calculation", "text": "15% of 240"}` → `201 {"id": "notebook:12", "title": "15% of 240 = 36", "url": "/n/12"}`. The sum is worked out with your own settings and kept as a new notebook called Quick sums (a `title` names it otherwise) with one calculation cell, like "Into a notebook" on the start page; several lines are one calculation and the answer is the last one's. A line the calculator cannot work out is not kept: `400` with the reason. The same `request_id` again answers the same notebook and adds nothing; with another sum it answers `400` |

## Account and data

`/auth/*`, `/users`, `/settings`, `/tokens`, `/export.json`, `/import` (the same file twice adds nothing twice; the counts come back under `added` and at the top level, with `skipped`), `/backup` (admins: the whole database), `/healthz`, `/readyz`, `/metrics` (with `METRICS_ENABLED`).

Pocket Move can copy your notebooks, calculations, kept answers and practice out and bring them in: `GET /pocket/portable` describes `/export.json` and `/export.csv` (link scope `move:export`) and `/import` (`move:import`).
