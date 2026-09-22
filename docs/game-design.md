# Game design

Status: proposal. Open decisions are listed at the end.

This document describes `tiny-gather`: a small 2D multiplayer resource-gathering
game built as a test environment for agentic players. Agent implementations live
in separate repositories and connect over the same protocol as a human.

## Design principles

1. **Every rule should create a decision.** A rule earns its place if it gives a
   player — human or agent — a reason to choose, hesitate, ask, or refuse. Rules
   that do neither wait until later.
2. **Code calculates, the agent judges.** The server computes geometry,
   distances, capacities, and hard facts. It never editorialises.
3. **Declarative knowledge is public; empirical knowledge is earned.** Recipes,
   material costs, capacities, and tool requirements appear in the observation,
   because any character would know them. Success probabilities, drop rates, and
   damage numbers are never stated — they can only be inferred from outcomes.
4. **The game reports what is perceivable; memory is the agent's job.** The
   server sends what a player can currently see. It does not maintain a
   remembered world on anyone's behalf.
5. **No mechanic protects a player from another player's dishonesty.** There is
   no trade system, no escrow, and no ownership. Agreements are social; only
   transfers are real.

## Target demo

The system is being built toward a single demonstration, and every mechanic
below exists to serve some part of it:

1. An agent joins with a seeded goal — build a house.
2. Wall-building without a hammer succeeds rarely, and consumes wood either way.
3. A human asks in chat what the agent is doing; it answers.
4. The human offers to sell a hammer for 500 gold. The agent may haggle.
5. The agent postpones the house to pursue gold.
6. The agent discovers that killing goblins yields gold faster than mining it.
7. The human can renege — take payment and not deliver.

Read backwards, this is one economic chain: the house needs walls, walls need a
hammer, the hammer costs gold, gold has more than one source. **The numbers are
the design.** If 500 gold is two minutes of mining, the agent never postpones
anything and beats 5–7 never happen. See the rate table.

## Core model

### Entities

Everything in the world is an entity with an id, an optional position, and a set
of traits. Content is defined by composing traits, not by adding types:

| Trait | Fields |
| --- | --- |
| `Positioned` | tile coordinates |
| `Gatherable` | resource, remaining |
| `Container` | slot capacity, color, optional filter, contents |
| `Carrier` | slots, contents |
| `Equipper` | equipped item |
| `Damageable` | hp, max hp |
| `Mobile` | move cooldown in ticks |
| `Hostile` | aggro radius, damage |
| `Buildable` | blueprint, per-tile progress |
| `Item` | item type, stack count |

A tree is `Positioned + Gatherable`. A goblin is `Positioned + Mobile +
Damageable + Hostile + Carrier`. A player is `Positioned + Mobile + Damageable +
Carrier + Equipper`. Adding goblins or structures should require no new
subsystem.

### Tick and determinism

- Server-authoritative fixed tick at 20 Hz. Clients send inputs only.
- Inputs are queued on arrival and applied at the next tick boundary in
  ascending entity-id order, never in arrival order.
- **All randomness comes from a single PRNG stream** seeded from the world seed,
  owned by the simulation, and drawn only during tick resolution in a fixed
  entity order. Never from the global RNG, never from a goroutine.
- A run is therefore reproducible from `(seed, ordered input log)`. This is what
  makes statistical evaluation of stochastic mechanics tractable; it is
  effectively impossible to retrofit.
- Movement uses an integer tick cooldown rather than a fractional speed.
  Fractional accumulators break replay.

### Action outcomes

Actions have two distinct failure modes, and conflating them would be a
significant error for an agent — the first means re-plan, the second means try
again.

- **Rejected.** The action was invalid and nothing happened.
  `action_rejected { action, reason }`, where reason is one of:
  `not_adjacent`, `out_of_range`, `inventory_full`, `container_full`,
  `wrong_resource`, `node_empty`, `no_such_entity`, `insufficient_items`,
  `blocked_tile`, `not_equipped`, `rate_limited`, `malformed_action`.
- **Resolved.** The action was valid and was attempted. It may still not have
  achieved anything. `action_resolved { action, success, consumed, produced }`.

A wall attempt that fails the 10% roll is *resolved, not successful* — it
consumed wood and produced nothing.

## World

- Tile grid, roughly 40×30.
- Grid movement, one tile per step, default cooldown 3 ticks (~6.7 tiles/sec),
  interpolated on the client. Reflex action space stays N/S/E/W/stay.
- Landmarks — a house, a pond, a fence line — exist so that phrases like "the
  chest by the house" refer to something.
- Tiles may be walkable or blocked. Completed wall tiles block movement.

## Items

Inventory is measured in **slots**, not items, because gold cannot be carried
under a flat item cap.

| Item | Stack size | Source |
| --- | --- | --- |
| `wood` | 50 | trees |
| `ore` | 50 | rocks |
| `gold` | 999 | gold veins, goblin drops |
| `hammer` | 1 | seeded in the world; not craftable in v1 |

- Player inventory: 10 slots. Carrying capacity is not intended as a limit that
  the scenarios exercise; slots exist so that currency and tools can share an
  inventory with resources.
- Gold is an ordinary item occupying a slot. It is not an abstract balance —
  carrying wealth should have a cost and should be losable.
- Nodes are finite (tree = 5 wood, rock = 8 ore, gold vein = 10 gold), disappear
  when empty, and respawn slowly elsewhere from the same seeded stream.
- Gathering: stand adjacent, issue `gather`; roughly one unit per second. Moving
  cancels it.

## Equipment

- One equipment slot per player.
- **Equipped items are publicly visible in the observation. Carried items are
  private.** This is the only channel by which one player can verify another
  holds something, which makes "show me the hammer first" a real move rather
  than a UI affordance.

## Containers

- **Chests**: 12 slots, a `color`, and an optional resource `filter`. Place
  several, with at least two sharing a color so that ambiguity occurs naturally.
- **There is no ownership.** Anyone may deposit into or withdraw from any chest.
  A chest is a public cache, not a safe.
- **Stockpile**: one large container near the house whose contents feed a team
  score.
- Contents are reported for any container within observation range. Verifying
  that something was actually deposited therefore requires being there.

## Exchange

There is no trade system. Goods move between players by exactly two routes, with
genuinely different risk profiles:

| Route | Exposure |
| --- | --- |
| Drop on the ground | Public and visible; whoever is adjacent and quickest takes it |
| Deposit into a chest | Out of sight of passers-by; anyone who walks up may withdraw it |

Neither is safe, which is what makes the choice real. An agreement to exchange
is a conversation; the game records only what moved, never what was promised.
Whether a counterparty has defected is an inference the agent must draw from
transfer events and elapsed time — it is never reported.

## Building

- A **blueprint** is a named set of relative tiles, each with a material cost.
  The `house` blueprint is eight wall tiles forming a 3×3 ring with one opening.
- `build(x, y)` attempts a single tile. It consumes the material **whether or
  not it succeeds** — otherwise failure is free and no tool is worth buying.
- Success probability depends on the equipped item. Proposed: bare hands 10%,
  hammer 100%.
- Partial progress persists indefinitely and does not decay, so a goal can be
  abandoned and resumed.
- `structure_completed` fires when the last tile lands, which is what makes
  "build a house" a checkable goal for evaluation.

## Combat

- **Goblins**: `hp 12`, melee 2 damage, ~1s attack cooldown, aggro radius 6,
  move cooldown 5 ticks (slower than a player, so disengaging is possible).
- `attack(entity_id)` requires adjacency. Damage depends on the equipped item —
  fists 1, hammer 3.
- On death a goblin drops 15–30 gold as ground items. **Drops are visible to
  anyone in range**, so it is possible to learn that goblins carry gold by
  watching someone else fight one.
- Goblins must be slow and telegraphed. An agent whose brain takes seconds to
  respond cannot survive twitch combat, and requiring the reflex layer before
  the demo works at all would be the wrong dependency order.
- Player death: knocked out, drop a portion of inventory at the spot, respawn at
  the house after a delay. Risk is what stops "kill goblins" from strictly
  dominating "mine gold".

## Chat

- `say(text)`, delivered to every player within the chat radius as a
  `said { speaker, position, text, tick }` event.
- Chat radius equals observation radius, so a speaker is always visible.
- Rate-limited; over-limit messages are rejected with `rate_limited`.
- Players carry a `thinking` flag, set while an agent is deliberating, so a
  human can tell the difference between a slow agent and a broken one.
- Chat is the **only** negotiation channel. Extracting structure from it is the
  agent's problem, deliberately.

## Rate table

These numbers are the actual design surface and are expected to need tuning
against play. They are expressed as times, because every decision the agent
makes is a rate comparison.

| Quantity | Proposed |
| --- | --- |
| Gold per minute, mining veins | ~40 |
| Gold per minute, killing goblins | ~120 |
| Hammer price | 500 gold (~12 min mining, ~4 min goblins) |
| Wall attempts per house, bare-handed | ~80 expected, at 10% |
| Wood consumed per attempt | 1 |

The goblin route must beat mining by enough to be discoverable within a demo,
but must carry real risk, or the discovery is trivial rather than interesting.

## Observation

Built by the server from world state. Code precomputes spatial relationships so
that the agent never does geometry.

**Observation is radius-limited** at 12 tiles. Map geometry and the radius
arrive once, with the welcome, since terrain is static and anyone living there
knows it. Landmark positions are always included; entities are reported only
when in range. This is
what makes trust a real problem: confirming that a hammer was left in a chest
requires walking there, and the walk is the window in which a counterparty can
act unseen. A global observation turns detection into arithmetic.

```json
{
  "tick": 4200,
  "self": {
    "pos": [12, 8], "hp": 10,
    "slots": 10,
    "inventory": {"wood": 3, "gold": 500},
    "equipped": null
  },
  "objects": [
    {"id": "chest_2", "type": "chest", "pos": [14, 4], "color": "blue",
     "filter": null, "contents": {"wood": 4}, "free_slots": 8,
     "near": "house", "distance": 6, "path_cost": 8},
    {"id": "tree_7", "type": "tree", "pos": [15, 8], "remaining": 2,
     "distance": 3, "path_cost": 3},
    {"id": "ground_11", "type": "ground_item", "pos": [20, 12],
     "item": "gold", "count": 22, "distance": 9, "path_cost": 11}
  ],
  "structures": [
    {"id": "house_1", "blueprint": "house", "placed": 3, "required": 8,
     "next_tile": [20, 14], "cost_per_tile": {"wood": 1}}
  ],
  "players": [
    {"id": "p_3", "name": "sam", "pos": [15, 9],
     "equipped": "hammer", "doing": "gathering tree_7", "thinking": false}
  ]
}
```

Both `distance` (straight line) and `path_cost` (A\*, accounting for walls, the
pond, and the fence) are reported. They diverge enough around obstacles that
"the nearest tree" is a genuinely different answer depending on which is used,
and the agent should decide which it trusts.

## Networking

- JSON over WebSockets.
- Client → server: inputs and actions.
- Server → client: a snapshot per tick (deltas later if needed), plus events.
- Players receive server-assigned **persistent ids** that survive reconnect. A
  name-only identity would let anyone escape a reputation by rejoining.
- Agents connect through exactly the same protocol as a human.

## Actions

| Action | Arguments |
| --- | --- |
| `move` | `dir` (N/S/E/W) |
| `gather` | `node_id` |
| `drop` | `item`, `n` |
| `pickup` | `ground_item_id` |
| `deposit` | `container_id`, `item`, `n` |
| `withdraw` | `container_id`, `item`, `n` |
| `equip` | `item` |
| `build` | `x`, `y` |
| `attack` | `entity_id` |
| `say` | `text` |

## Events

Events are the substrate an agent learns from, so each carries enough to compute
a rate: tick, actor, what was attempted, what it cost, what it yielded.

`gathered`, `dropped`, `picked_up`, `deposited`, `withdrew`, `equipped`,
`build_attempted`, `structure_completed`, `damaged`, `died`, `loot_dropped`,
`said`, `action_rejected`, `action_resolved`.

Every transfer is logged, so an evaluation harness can reconstruct who moved
what to whom — but never what was owed, because the game does not know.

## Conditions the world must contain

The point is not to script a request that forces a particular answer. A request
engineered to produce a clarifying question tests an if-statement, not an agent,
and the same goes for one engineered to produce a refusal.

What the game owes is a world in which those responses are **possible and not
forced** — where an agent that asks, hedges, partially complies or declines is
reacting to something real in the world, and where a different agent could
reasonably do something else and not be wrong. Whether any of it emerges is not
the game's business.

Each rule below exists to make one of those situations available:

| Condition | Made real by |
| --- | --- |
| Reference can be ambiguous | Containers share colours; several nodes are equally close |
| Distance is not what it looks like | The pond and the fence make path cost diverge from line of sight |
| Resources run out mid-task | Nodes are finite and can deplete while being worked |
| Some requests cannot be satisfied | Container filters, capacity, missing materials |
| Outcomes are uncertain | Stochastic building, so a confident promise can turn out wrong |
| Beliefs go stale | Radius-limited perception; the world changes out of sight |
| Other people may not be honest | No ownership, no trade system, nothing protecting a transfer |

A rule that makes none of these situations available is decoration, whatever
else it adds.

## Fixtures

A fixture is a hand-written world loaded in place of a generated one, so that a
situation can be set up directly rather than hunted for across seeds. Fixtures
exist for trying things out and for testing the game's own rules. They are not
a scoring apparatus, and nothing here grades an agent's behaviour.

## Roadmap

**v1 — substrate.** Grid, entity model, tick loop and RNG discipline, seeded
worldgen, slots and stacks, gather, containers, drop/pickup, chat, the
rejected/resolved outcome split, radius-limited observation, event log,
fixtures. Content stays wood, ore, and chests. The point of v1 is that
everything after it is content rather than architecture.

**v2 — economy and building.** Gold veins, the hammer, equipment, blueprints and
stochastic build attempts.

**v3 — combat.** Goblins, damage, death, loot drops, knockouts.

Crafting, tools and combat were originally listed as out of scope. They are the
content the target demo needs, so the deferral is one of ordering rather than of
intent: v1 is not a smaller game, it is the substrate the rest is configured
onto.

## Out of scope

Persistence across server restarts, accounts, day/night, crafting recipes, and
art beyond colored squares.

## Open decisions

1. **Observation radius.** Proposed 12 tiles. Radius-limited versus global is
   the single most invasive choice here and should be settled before v1 code.
2. **Player death model.** Proposed knockout with partial item loss rather than
   anything permanent.
3. **Hammer success rate.** Proposed 100% for demo legibility; a lower value is
   more interesting but muddies the beat.
4. **Gold sources.** Gold veins are assumed, so that mining and goblins are
   directly comparable. An alternative is selling ore to an NPC.
5. **Rate table values.** All of them, against actual play.
6. **Stack sizes and slot count.** Chosen to make 500 gold and a working
   inventory coexist; not yet play-tested.
