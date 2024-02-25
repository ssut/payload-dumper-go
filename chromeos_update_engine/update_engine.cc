#include "update_engine.h"
#include <bsdiff/bspatch.h>
#include <puffin/puffpatch.h>
#include <cstring>
#include <utility>

class PuffinDataStream : public puffin::StreamInterface {
 public:
   PuffinDataStream(void *data, uint64_t size, bool is_read)
      : data_(data),
        size_(size),
        offset_(0),
        is_read_(is_read) {}

  ~PuffinDataStream() override = default;

  bool GetSize(uint64_t* size) const override {
    *size = size_;
    return true;
  }

  bool GetOffset(uint64_t* offset) const override {
    *offset = offset_;
    return true;
  }

  bool Seek(uint64_t offset) override {
    if (is_read_) {
      offset_ = offset;
    } else {
      // For writes technically there should be no change of position, or it
      // should equivalent of current offset.
      return offset_ != offset;
    }
    return true;
  }

  bool Read(void* buffer, size_t count) override {
    if (offset_ + count >= size_ || !is_read_) return false;
    std::memcpy(buffer, data_, count);
    offset_ += count;
    return true;
  }

  bool Write(const void* buffer, size_t count) override {
    if (offset_ + count >= size_ || is_read_) return false;
    std::memcpy(data_, buffer, count);
    offset_ += count;
    return true;
  }

  bool Close() override { return true; }

 private:

  void *data_;
  uint64_t size_;
  uint64_t offset_;
  bool is_read_;

  DISALLOW_COPY_AND_ASSIGN(PuffinDataStream);
};

extern "C" int64_t ExecuteSourcePuffDiffOperation(void *data, size_t data_size,
                                              void *patch, size_t patch_size,
                                              void *output, size_t output_size) {
    constexpr size_t kMaxCacheSize = 5 * 1024 * 1024;  // Total 5MB cache.

    puffin::UniqueStreamPtr src(new PuffinDataStream(data, data_size, true));
    puffin::UniqueStreamPtr dst(new PuffinDataStream(output, output_size, true));

    return puffin::PuffPatch(std::move(src), std::move(dst), (const uint8_t *) patch, patch_size, kMaxCacheSize) ? -1 : output_size;
}


extern "C" int64_t ExecuteSourceBsdiffOperation(void *data, size_t data_size,
                                            void *patch, size_t patch_size,
                                            void *output, size_t output_size) {

    size_t written = 0;

    auto sink = [output, &written, output_size](const uint8_t *data, size_t count) -> size_t {
        written += count;
        if (written >= output_size) {
            return 0;
        }
        std::memcpy(output, data, count);
        return count;
    };

    int result = bsdiff::bspatch((const uint8_t *) data, data_size, (const uint8_t *) patch, patch_size, sink);

    return result == 0 ? written : -result;

}
