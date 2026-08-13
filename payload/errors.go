package payload

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrNotPayload = errors.New("payload: input is neither a payload.bin (magic \"CrAU\") nor a zip archive containing payload.bin")

type UnsupportedOperationsError struct {
	Partitions map[string][]string
}

func (e *UnsupportedOperationsError) Error() string {
	names := make([]string, 0, len(e.Partitions))
	for name := range e.Partitions {
		names = append(names, name)
	}
	sort.Strings(names)
	details := make([]string, 0, len(names))
	for _, name := range names {
		details = append(details, fmt.Sprintf("%s (%s)", name, strings.Join(e.Partitions[name], ", ")))
	}
	return fmt.Sprintf("payload: partitions use install operations that are not supported yet: %s", strings.Join(details, "; "))
}

type MissingSourceError struct {
	Partitions []string
}

func (e *MissingSourceError) Error() string {
	return fmt.Sprintf("payload: this is a delta (incremental) payload; source images are required for: %s", strings.Join(e.Partitions, ", "))
}

type UnknownPartitionsError struct {
	Names []string
}

func (e *UnknownPartitionsError) Error() string {
	return fmt.Sprintf("payload: requested partitions not present in payload: %s", strings.Join(e.Names, ", "))
}

type VerificationError struct {
	Partition string
	Subject   string
	Expected  string
	Actual    string
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("payload: %s verification failed for partition %q: expected sha256 %s, got %s", e.Subject, e.Partition, e.Expected, e.Actual)
}
