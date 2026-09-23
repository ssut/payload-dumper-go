package payload

const PhaseFEC = "fec"

type ProgressEvent struct {
	Partition      string
	TotalOps       int
	CompletedOps   int
	Phase          string
	PhaseCompleted int
	PhaseTotal     int
	Done           bool
	Err            error
}

// ProgressFunc must be safe for concurrent use: events arrive from one
// goroutine per partition, and from several FEC workers within a partition.
type ProgressFunc func(ProgressEvent)
