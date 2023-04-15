package rxd

import "fmt"

type serviceNotify struct {
	services []*Service
}

func (n *serviceNotify) notify(state State, logChannel chan LogMessage) {
	for _, service := range n.services {
		// to ensure we dont attempt to send over a close stateC for services that might have stopped.
		if !service.ctx.isShutdown {
			service.ctx.stateC <- state
			logChannel <- NewLog(fmt.Sprintf("%s was informed of %s state change", service.Name(), state), Debug)
		}
	}
	logChannel <- NewLog(fmt.Sprintf("%s services notifying complete", state), Debug)
}
