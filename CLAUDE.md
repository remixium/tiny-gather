# tiny-gather

A small 2D multiplayer resource-gathering game, built as an environment for
agentic players. **This repository contains the game only.**

It is a personal project, and a step toward robotics rather than toward a
benchmark. The interesting parts are embodiment and partial observability: an
agent perceives only what is near it, has to keep its own model of everything
else, and acts through the same limited interface a human does. Nothing here
scores an agent or compares implementations, and no scenario should be built to
force a particular response — the game's job is to make interesting responses
possible, not to elicit them.

See `docs/game-design.md` for the full design, the target demo that drives it, and
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

Phase 3 is done: the game is playable in a terminal. A server, a human client
and the protocol they share all exist; what is missing is the Ebiten renderer
and anything past v1 content.

```
make          # list targets
make ci       # everything CI runs: fmt, tidy, vet, build, race tests
go run ./cmd/tg-server -addr localhost:8080 -seed 42
go run ./cmd/tg-server -fixture fixtures/two-blue-chests.json
go run ./cmd/tg-play -name ana            # in another terminal
go run ./cmd/tg-play -token <token>       # printed on quit; reattaches
```

In the client: WASD or arrows move, `g` gathers the node you stand beside, `p`
picks up, `t` or Enter opens chat, `:` opens a command line that mirrors the
protocol one to one (`deposit chest_2 wood 3`, `withdraw`, `drop`, `gather`,
`pickup`, `say`, `move`), `q` quits.

Layout, where `*` marks what exists today:

```
* cmd/tg-server/      authoritative server
* cmd/tg-play/        human client (terminal now, Ebiten later)
* pkg/protocol/       wire types — public; the contract agents code against
* internal/rng/       the one deterministic random source
* internal/world/     grid, entities, seeded worldgen, fixtures, path costs
* internal/rules/     action validation and outcomes
* internal/sim/       tick loop, deterministic ordering, RNG ownership
* internal/observe/   observation builder
* internal/wsserver/  transport
* client/conn/        reusable protocol client
* client/render/term/ terminal renderer
  client/render/      Ebiten renderer, later, on the same conn package
* fixtures/           hand-written worlds
* docs/               design
```

Only `protocol` is public. Everything else stays `internal/`.

Actions implemented so far are `move`, `gather`, `drop`, `pickup`, `deposit`,
`withdraw` and `say`. Building, equipment and combat arrive with v2 and v3; the
reason codes `not_equipped` and `out_of_range` are reserved for them and are not
yet produced.

The terminal renderer shows exactly what the server sent. Terrain is drawn
everywhere because it arrives with the welcome; entities only where the current
frame reports them; tiles beyond perception range are left blank. There is
deliberately no client-side memory of things seen earlier — what a human sees
is what an agent gets.

## Concurrency

The simulation is not safe for concurrent use, and making it so would invite
the nondeterminism the design rules out. Every piece of simulation state is
owned by the goroutine running `wsserver.Server.Run`. Connections read frames
on their own goroutines and hand them over through channels; nothing else
touches the world. Commands are drained at tick boundaries, never applied
mid-tick.

A live server is not itself reproducible, because network timing decides which
tick an action lands on. Reproducibility belongs to replay: a recorded seed and
ordered input log driven through the simulation directly.

## Invariants

These were decided deliberately and are expensive to recover once broken. Do not
change them without saying so explicitly.

**Determinism.** A run must be reproducible from `(seed, ordered input log)`.
This is for debugging and because replayable simulation is ordinary robotics
practice — not for measurement. With a language model and a probabilistic
decision layer both in play, the world is the one source of randomness that can
be held still, which is what makes the others possible to study.

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

## Fixtures

A fixture is a hand-written world loaded in place of a generated one, so a
situation can be set up directly rather than hunted for across seeds. They are
for trying things out and for testing the game's own rules. They are not a
scoring apparatus.
