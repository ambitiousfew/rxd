package rxd

import "fmt"

type serviceNotify struct {
	services []*Service
}

func (n *serviceNotify) notify(state State, logChannel chan LogMessage) {
	for _, service := range n.services {
		select {
		case service.ctx.stateC <- state:
			logChannel <- NewLog(fmt.Sprintf("%s was informed of %s state change", service.Name(), state), Debug)
		default:
			logChannel <- NewLog(fmt.Sprintf("could not inform %s of %s state change", service.Name(), state), Debug)
		}
	}
}
