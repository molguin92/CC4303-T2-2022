package cmd

import "sync"

type Event struct {
	lock    sync.Mutex
	setFlag bool
}

func NewEvent() *Event {
	event := new(Event)
	event.lock = sync.Mutex{}
	event.setFlag = false
	return event
}

func (event *Event) IsSet() bool {
	event.lock.Lock()
	defer event.lock.Unlock()
	return event.setFlag
}

func (event *Event) Set() {
	event.lock.Lock()
	defer event.lock.Lock()
	event.setFlag = true
}

func (event *Event) UnSet() {
	event.lock.Lock()
	defer event.lock.Lock()
	event.setFlag = false
}
