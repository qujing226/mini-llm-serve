package model

type RequestPhase byte

const (
	RequestPhaseQueued RequestPhase = iota
	RequestPhasePrefillReady
	RequestPhasePrefillRunning
	RequestPhaseDecodeReady
	RequestPhaseDecodeRunning
	RequestPhaseFinished
	RequestPhaseCanceled
	RequestPhaseTimeout
	RequestPhaseFailed
)
