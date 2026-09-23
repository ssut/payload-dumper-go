package payload

func WithFECBatchRounds(opts ExtractOptions, rounds int) ExtractOptions {
	opts.fecBatchRounds = rounds
	return opts
}
