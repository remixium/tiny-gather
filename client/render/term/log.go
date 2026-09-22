package term

import (
	"fmt"

	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// entry is one line in the side panel's log.
type entry struct {
	text  string
	style tcell.Style
}

// describe turns an event into a log line, or "" for events not worth one.
//
// Resolutions of ordinary actions are silent when they succeed, since the world
// view already shows the result; they are reported when they do not, because an
// action that was valid and still produced nothing is exactly the thing a player
// needs to be told.
func (u *ui) describe(ev protocol.Event) (string, tcell.Style) {
	self := u.frame.Observation.Self.ID
	who := u.name(ev.Actor)

	switch ev.Kind {
	case protocol.EvSaid:
		return fmt.Sprintf("%s: %s", who, ev.Text), styleChat

	case protocol.EvRejected:
		if ev.Actor != self {
			return "", styleBase
		}
		return fmt.Sprintf("x %s: %s", ev.Action, ev.Reason), styleBad

	case protocol.EvResolved:
		if ev.Actor != self || ev.Succeeded() {
			return "", styleBase
		}
		return fmt.Sprintf("- %s stopped", ev.Action), styleWarn

	case protocol.EvGathered:
		if ev.Actor != self {
			return "", styleBase
		}
		return fmt.Sprintf("+%d %s", ev.N, ev.Item), styleGood

	case protocol.EvDeposited:
		return fmt.Sprintf("%s put %d %s in %s", who, ev.N, ev.Item, ev.Target), styleBase
	case protocol.EvWithdrew:
		return fmt.Sprintf("%s took %d %s from %s", who, ev.N, ev.Item, ev.Target), styleBase
	case protocol.EvDropped:
		return fmt.Sprintf("%s dropped %d %s", who, ev.N, ev.Item), styleBase
	case protocol.EvPickedUp:
		return fmt.Sprintf("%s picked up %d %s", who, ev.N, ev.Item), styleBase

	case protocol.EvNodeDepleted:
		return fmt.Sprintf("%s is exhausted", ev.Target), styleWarn
	}
	return "", styleBase
}
