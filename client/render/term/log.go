package term

import (
	"fmt"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// describe turns an event into a log line, or "" for events not worth a line.
//
// Resolutions of ordinary actions are silent when they succeed, since the world
// view already shows the result; they are reported when they do not, because an
// action that was valid and still produced nothing is exactly the thing a player
// needs to be told.
func (u *ui) describe(ev protocol.Event) string {
	self := u.frame.Observation.Self.ID
	who := u.name(ev.Actor)

	switch ev.Kind {
	case protocol.EvSaid:
		return fmt.Sprintf("%s: %s", who, ev.Text)

	case protocol.EvRejected:
		if ev.Actor != self {
			return ""
		}
		return fmt.Sprintf("x %s: %s", ev.Action, ev.Reason)

	case protocol.EvResolved:
		if ev.Actor != self || ev.Succeeded() {
			return ""
		}
		return fmt.Sprintf("- %s stopped", ev.Action)

	case protocol.EvGathered:
		if ev.Actor != self {
			return ""
		}
		return fmt.Sprintf("+%d %s", ev.N, ev.Item)

	case protocol.EvDeposited:
		return fmt.Sprintf("%s put %d %s in %s", who, ev.N, ev.Item, ev.Target)
	case protocol.EvWithdrew:
		return fmt.Sprintf("%s took %d %s from %s", who, ev.N, ev.Item, ev.Target)
	case protocol.EvDropped:
		return fmt.Sprintf("%s dropped %d %s", who, ev.N, ev.Item)
	case protocol.EvPickedUp:
		return fmt.Sprintf("%s picked up %d %s", who, ev.N, ev.Item)

	case protocol.EvNodeDepleted:
		return fmt.Sprintf("%s is exhausted", ev.Target)
	case protocol.EvNodeRespawned:
		return ""
	}
	return ""
}
