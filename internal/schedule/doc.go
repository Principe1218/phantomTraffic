// Package schedule evaluates time-of-day on/off windows for the engine's
// scheduler. It is a pure, clock-agnostic layer over the FROZEN
// scenario.Schedule type: Active reports whether an instant falls inside any
// configured window, and NextTransition reports the next instant the
// active/inactive state flips at or after a given time. Windows evaluate in the
// schedule's *time.Location; an empty Windows slice means "always active".
//
// The engine's scheduler goroutine drives these functions with its injected
// clock.Clock (design §5, §6.4), but the functions take an explicit time.Time so
// they remain deterministic and unit-testable with fixed instants. There are no
// goroutines, no I/O, and no hidden clock here.
package schedule
