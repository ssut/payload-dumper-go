package payload

type ProgressEvent struct {
	Partition    string
	TotalOps     int
	CompletedOps int
	Done         bool
	Err          error
}

type ProgressFunc func(ProgressEvent)
