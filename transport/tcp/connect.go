package tcp

// maxSegmentsPerWake is the maximum number of segments to process in the main
// protocol goroutine per wake-up. Yielding [after this number of segments are
// processed] allows other events to be processed as well (e.g., timeouts,
// resets, etc.)
const maxSegmentsPerWake = 100

// The following are used to set up sleepers
const (
	wakerForNotification = iota
	wakerForNewSegment
	wakerForResend
)
