package tui

type linkEnterAction uint8

const (
	linkEnterNone linkEnterAction = iota
	linkEnterGo
	linkEnterRefuse
)

type linkActions struct {
	enter  linkEnterAction
	finger bool
	copy   bool
}

func actionsForLink(link Link) linkActions {
	actions := linkActions{copy: true}
	if link.Blocked != "" {
		actions.enter = linkEnterRefuse
		return actions
	}
	if link.Kind != LinkFinger || link.Target.HostPort == "" {
		return actions
	}
	if link.Ambiguous {
		actions.finger = true
		return actions
	}
	if link.Action == ActionDrill {
		actions.enter = linkEnterGo
	}
	return actions
}
