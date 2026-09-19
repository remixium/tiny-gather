# tiny-gather

A small 2D multiplayer resource-gathering game, built as a test environment for
agentic players. **This repository contains the game only.**

See `game-design.md` for the full design, the target demo that drives it, and
the list of open decisions.

## Who consumes this

Agent implementations live in **separate repositories** and connect over exactly
the same WebSocket protocol as a human client. No agent code, model integration,
or API credentials belong here — if a task seems to call for them, it belongs in
the agent repo instead.

The consequence for this codebase is that the observation the server builds is
fed to language and typed-decision models, so it must stay compact and
machine-readable. Watch its serialized size; agents pay per token.

## Stack

- Go for both the server and the client.
- Client renders in a terminal first (tcell), with an Ebiten renderer added on
  top of the same protocol client afterward.
- JSON over WebSockets.

## Status

The repository is **documentation only** at present. No `go.mod`, no source, no
tests. Build and test commands land with the first phase of implementation.

Planned layout:

```
cmd/tg-server/      authoritative server
cmd/tg-play/        human client (terminal, then Ebiten)
cmd/tg-scenario/    headless scripted client and scenario runner
pkg/protocol/       wire types — public; this is the contract agents code against
internal/world/     grid, entities, seeded worldgen
internal/rules/     action validation and outcomes
internal/sim/       tick loop, deterministic ordering, RNG
internal/observe/   observation builder
internal/wsserver/  transport
client/conn/        reusable protocol client
client/render/      terminal and Ebiten renderers
fixtures/           scenario world layouts
```

Only `protocol` is public. Everything else stays `internal/`.

## Invariants

These were decided deliberately and are expensive to recover once broken. Do not
change them without saying so explicitly.

**Determinism.** A run must be reproducible from `(seed, ordered input log)`.

- All randomness comes from one PRNG stream, seeded from the world seed, owned
  by `sim`, drawn only during tick resolution in a fixed entity order. Never the
  global `math/rand`, never from a goroutine.
- Inputs are applied at tick boundaries in ascending entity-id order, never in
  arrival order.
- Movement and timing use integer tick cooldowns. No fractional accumulators.

**Outcomes.** `action_rejected` (invalid, nothing happened) and
`action_resolved` (valid, attempted, may have failed anyway) are distinct and
must not be collapsed. An agent treats them completely differently: the first
means re-plan, the second means try again.

**Observation.** The server builds it, radius-limited. Code does all geometry and
pathfinding; agents never compute spatial relationships themselves. The server
reports what is currently perceivable and maintains no remembered world on
anyone's behalf.

**Information.** Declarative knowledge is public — recipes, costs, capacities,
tool requirements. Empirical knowledge is earned — success probabilities, drop
rates, and damage numbers never appear in the observation. Equipped items are
visible to others; carried items are private.

**Exchange.** There is no ownership, no trade system, and no escrow. Goods move
only by ground drops and chests, so agreements are social and defection is
possible. The game records what moved, never what was promised.

**Identity.** Player ids are server-assigned and persist across reconnect.

**Events.** Every transfer and outcome is logged with a tick stamp and enough
detail to compute a rate. Events are the substrate agents learn from.

## Evaluation

Scenarios in `game-design.md` double as a reproducible test set: a fixed world
layout, a seed, and a scripted counterparty. Once outcomes are stochastic a
scenario is scored over N seeded runs rather than as a single pass/fail, which
is why the determinism invariants matter.
