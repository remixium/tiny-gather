# AI-PC

An autonomous AI teammate that plays a small 2D multiplayer resource-gathering game alongside humans. It's an experiment in giving an embodied agent a System 2 / System 1 split: an LLM for slow, strategic thinking and TypeSafe's Jev model for fast, typed decisions.

The long-term goal: a player can ask the AI-PC something like "go gather some wood and put it in the blue chest next to the house," and it will **comply**, **partially comply** (with caveats), **ask a clarifying question**, or **refuse** (with a justification). There is also interest in how this architecture transfers to robotics.

## Architecture

Three layers, plus deterministic code underneath:

1. **LLM brain (strategic, seconds).** Interprets requests (text now, voice later), breaks them into plans made of game skills, and produces all natural-language output: confirmations, caveats, clarifying questions, refusals.
2. **Jev tactical layer (a few Hz, called by the plan).** Answers questions the plan asks: which object a phrase refers to, which tree to go for, whether the current plan is still valid. Low confidence escalates to the LLM (e.g. to ask the player which chest they meant).
3. **Jev reflex layer (a few Hz, always running).** Reacts to the environment on its own, e.g. a hazard that just appeared, and can preempt the current plan, then hand control back so the LLM can re-plan. The LLM is too slow for this.
4. **Deterministic code.** Pathfinding (A*), game rules, spatial relationships ("near the house", distances), and executing actions. Jev never does geometry or checks hard facts like ownership or capacity; code does.

Guiding rule, from TypeSafe's own guidance: **code calculates, Jev judges, the LLM reasons and talks.**

## Jev (TypeSafe)

- Launched 2026-09-15; docs at https://docs.typesafe.ai (index: https://docs.typesafe.ai/llms.txt). The API is new and may change, so read the live docs before changing the integration. Install the TypeSafe skill in Claude Code:
  `claude plugin marketplace add typesafe-ai/skills` then `claude plugin install typesafe@typesafe-ai`.
- Endpoint: `POST https://api.typesafe.ai/v1/systemone` with `{state, model, questions}`. Python SDK: `typesafe-sdk` (`TypeSafeClient`, `AsyncTypeSafeClient`, `Choice`, `Noul`, `Score`).
- Question types: **Noul** (probability of yes, no confidence field), **Choice** (one option from a map; up to 255 options), **Score** (ordered levels, 2–10).
- Choice and Score answers carry `probabilities` and `confidence`. Use confidence thresholds to decide act / confirm / clarify, scaled to the stakes of the action. Thresholds must be tuned against the real model; don't trust values tuned on the mock.
- Text-only input. Jev does not generate text.
- Latency is roughly 70–500ms per call and pricing is per input token, so keep state compact and measure `usage.input_tokens` per tick.
- **The API key lives in `TYPESAFE_API_KEY`. Never commit it, log it, or send it to the game client.**

### Mock vs live

`aipc/jev/` contains a local mock that speaks the exact `/v1/systemone` contract, served to the real SDK through an in-process httpx transport. Agent code always uses the real SDK client; only the wire changes.

- `AIPC_JEV_MODE=mock` uses the mock (the default when no API key is set); `AIPC_JEV_MODE=live` calls the real API.
- The mock uses keyword-overlap heuristics so answers react to state, plus scripted overrides (`engine.script(...)`, `engine.script_when(...)`) to pin outcomes in tests.
- Use the mock for unit tests, offline work, and developing control flow. Validate behavior and tune thresholds against the live model.
- Run tests with `pytest`.

## Game

See `docs/game-design.md` for the proposed v1 scope: a tile grid, wood and ore, a shared-capacity inventory, colored and owned chests plus a shared stockpile, a minimal action set with machine-readable failure reasons, and a server-authoritative tick loop over WebSockets. The AI-PC connects through the same protocol as a human player.

## Open decisions

- **Stack.** Options discussed: Go game server + Python agent (current leaning in discussion), all Python, or all TypeScript. The Jev mock is Python.
- **Movement.** Grid movement is proposed (simple pathfinding, small reflex action space) but not confirmed.
- **Voice.** Planned as an input channel to the LLM brain, after the text path works. The agent is meant to be autonomous; requests are one input, not the only one.

## Evaluation

A secondary goal is comparison: run the same scenarios through Jev, an ordinary LLM constrained to Jev's format via TypeSafe's System One adapter, and scripted logic, measuring task success, latency, cost, and how often each correctly asks instead of guessing. Keep scenarios as fixed, reproducible test cases from the start (see the scenario list in the design doc).
