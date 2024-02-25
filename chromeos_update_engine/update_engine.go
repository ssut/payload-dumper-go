package chromeos_update_engine

//go:generate make -B -j libs

/*
#cgo CXXFLAGS: -Ibsdiff/include -Ipuffin/src/include
#cgo LDFLAGS: -labsl_log_internal_message -Lbuild -l:libpuffpatch.a -l:libbspatch.a -l:libzucchini.a -l:libbz2.a -l:libbrotli.a -l:libchrome.a -labsl_log_internal_check_op -lprotobuf
#include "update_engine.h"
*/
import "C"
import (
	"unsafe"
	"fmt"
)

func ExecuteSourceBsdiffOperation(data []byte, patch []byte) ([]byte, error) {
	output := make([]byte, len(data))
	result := int(C.ExecuteSourceBsdiffOperation(
						unsafe.Pointer(&data[0]), C.ulong(len(data)),
						unsafe.Pointer(&patch[0]), C.ulong(len(patch)),
						unsafe.Pointer(&output[0]), C.ulong(len(output)),
	))

	if result < 0{
		return nil, fmt.Errorf("C++ ExecuteSourceBsdiffOperation call failed (returned %d)", result)
	}

	if (result != len(output)) {
		realloc := make([]byte, result)
		copy(realloc, output)
		return realloc, nil
	}

	return output, nil
}

func ExecuteSourcePuffDiffOperation(data []byte, patch []byte) ([]byte, error) {
	output := make([]byte, len(data))
	result := int(C.ExecuteSourcePuffDiffOperation(
						unsafe.Pointer(&data[0]), C.ulong(len(data)),
						unsafe.Pointer(&patch[0]), C.ulong(len(patch)),
						unsafe.Pointer(&output[0]), C.ulong(len(output)),
	))
	if result < 0{
		return nil, fmt.Errorf("C++ ExecuteSourcePuffDiffOperation call failed (returned %d)", result)
	}

	if (result != len(output)) {
		realloc := make([]byte, result)
		copy(realloc, output)
		return realloc, nil
	}

	return output, nil
}
