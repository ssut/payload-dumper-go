#ifndef BSDIFF4_IMPL_H
#define BSDIFF4_IMPL_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int64_t ExecuteSourceBsdiffOperation(void *data, size_t data_size,
                                     void *patch, size_t patch_size,
                                     void *output, size_t output_size);

int64_t ExecuteSourcePuffDiffOperation(void *data, size_t data_size,
                                       void *patch, size_t patch_size,
                                       void *output, size_t output_size);



#ifdef __cplusplus
}
#endif

#endif /* BSDIFF4_IMPL_H */
