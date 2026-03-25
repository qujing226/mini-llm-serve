package scheduler

type Scheduler interface {
}

type scheduler struct {
}

func NewScheduler() Scheduler {
	return new(scheduler)
}
