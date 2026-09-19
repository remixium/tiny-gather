# Game design: proposed v1 scope

Status: proposal. Stack and movement model are not yet confirmed (see CLAUDE.md).

Design principle: every rule should either create a decision for Jev or give the agent a reason to comply partially, ask, or refuse. Rules that do neither wait until later.

## World

- Tile grid, roughly 40×30.
- Proposed: grid movement, one tile per step (WASD, ~6 tiles/sec, interpolated on the client). Keeps pathfinding simple and the reflex action space small (N/S/E/W/stay).
- Server-authoritative fixed tick (~20 Hz). Clients send inputs only.
- Landmarks for reference: a house, a pond, a fence line. They exist so phrases like "the chest by the house" mean something.

## Resources

- **Wood** from trees, **ore** from rocks.
- Nodes are finite (e.g. tree = 5 wood, rock = 8 ore), disappear when empty, and respawn slowly elsewhere.
- Gathering: stand adjacent and hold interact; about one unit per second; moving cancels.

## Inventory

- One total capacity per player: 10 items in any mix of wood and ore.

## Deposit places

- **Chests**: capacity 12; each has a `color`, an `owner` (a player name or `"shared"`), and an optional resource `filter` (e.g. ore only). Place two or three, with at least two sharing a color so ambiguity occurs naturally.
- **Stockpile**: one large shared container near the house that feeds a team score.
- **Ownership rule**: anyone can deposit into any chest; only the owner can withdraw from a private chest.

## Actions

| Action | Arguments |
| --- | --- |
| `move` | `dir` (N/S/E/W) |
| `gather` | `node_id` |
| `deposit` | `container_id`, `resource`, `n` |
| `withdraw` | `container_id`, `resource`, `n` |

Failed actions return a machine-readable reason:

`not_adjacent`, `inventory_full`, `container_full`, `wrong_resource`, `not_owner`, `node_empty`

The executor uses these to recover; the LLM turns them into explanations.

## Networking

- JSON over WebSockets.
- Client → server: inputs and actions.
- Server → client: a snapshot per tick (deltas later if needed), plus events: `gathered`, `deposited`, `withdrew`, `action_failed`.
- The AI-PC connects through exactly the same protocol as a human player.

## Agent observation

Built by code from the server state; this is also the `state` sent to Jev. Code precomputes spatial relationships so Jev never does geometry.

```json
{
  "self": {"pos": [12, 8], "inventory": {"wood": 3, "ore": 0}, "capacity": 10},
  "objects": [
    {"id": "chest_2", "type": "chest", "color": "blue", "owner": "shared",
     "contents": {"wood": 4}, "free": 8, "near": "house", "distance": 6},
    {"id": "chest_5", "type": "chest", "color": "blue", "owner": "sam",
     "contents": {}, "free": 12, "near": "pond", "distance": 14},
    {"id": "tree_7", "type": "tree", "remaining": 2, "distance": 3}
  ],
  "players": [{"name": "sam", "pos": [15, 9], "doing": "gathering tree_7"}]
}
```

## Scenarios

These double as the evaluation set. Each should be a reproducible fixture.

| Outcome | Example request | Rule that triggers it |
| --- | --- | --- |
| Comply | "Put 5 wood in the shared blue chest." | — |
| Partial | "Bring me 15 ore." | Inventory capacity |
| Partial | "Get wood from that tree." | Node has only 2 left |
| Clarify | "Put it in the blue chest." | Two blue chests |
| Refuse | "Take Sam's ore." | Ownership |
| Refuse | "Put wood in the ore bin." | Chest filter |

## Out of scope for v1

Crafting, tools, combat, day/night, persistence, accounts, and art beyond colored squares.

## Next rules to add

1. **A hazard** (e.g. a wandering animal that bumps players and knocks an item loose). The reflex layer needs something to react to.
2. **Tools** (ore requires a pickaxe). Adds a feasibility refusal.
