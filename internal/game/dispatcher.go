package game

import "time"

type Dispatcher struct {
}

func (d *Dispatcher) AddEvent(delay time.Duration, action func()) {
	time.AfterFunc(delay, action)
}

var GlobalDispatcher = &Dispatcher{}
