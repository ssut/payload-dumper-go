CXXFLAGS=-std=gnu++17
CFLAGS=-I. -Ibsdiff/include -Ipuffin/src/include -Izucchini/aosp/include -Ilibchrome -Igtest/include -I/usr/include/glib-2.0 -I/usr/lib/glib-2.0/include -Ibuild -ggdb
LDFLAGS=-Wl,--copy-dt-needed-entries -labsl_log_internal_check_op

libs: build build/libbrotli.a build/libbspatch.a build/libbz2.a build/libpuffpatch.a build/libzucchini.a build/libchrome.a

build:
	mkdir -p $@

build/%.o: %.cc
	@ mkdir -p $(dir $@)
	$(CXX) $(CFLAGS) $(CXXFLAGS) -c -o $@ $< $(LDFLAGS)

build/%.o: %.c
	@ mkdir -p $(dir $@)
	$(CC) $(CFLAGS) -c -o $@ $< $(LDFLAGS)

BSPATCH_SRC=bsdiff/brotli_decompressor.cc bsdiff/bspatch.cc bsdiff/bz2_decompressor.cc bsdiff/buffer_file.cc bsdiff/decompressor_interface.cc bsdiff/extents.cc bsdiff/extents_file.cc bsdiff/file.cc bsdiff/logging.cc bsdiff/memory_file.cc bsdiff/patch_reader.cc bsdiff/sink_file.cc bsdiff/utils.cc
BSPATCH_OBJ=$(addprefix build/,$(BSPATCH_SRC:.cc=.o))

build/libbspatch.a: $(BSPATCH_OBJ)
	$(AR) rcs $@ $^

BROTLI_SRC=$(wildcard brotli/c/common/*.c brotli/c/dec/*.c brotli/c/enc/*.c)
BROTLI_OBJ=$(addprefix build/,$(BROTLI_SRC:.c=.o))

build/libbrotli.a: $(BROTLI_OBJ)
	$(AR) rcs $@ $^

BZ2_SRC=bzip2/blocksort.c bzip2/huffman.c bzip2/crctable.c bzip2/randtable.c bzip2/compress.c bzip2/decompress.c bzip2/bzlib.c
BZ2_OBJ=$(addprefix build/,$(BZ2_SRC:.c=.o))

build/libbz2.a: $(BZ2_OBJ)
	$(AR) rcs $@ $^

PUFFPATCH_SRC=puffin/src/bit_reader.cc puffin/src/bit_writer.cc puffin/src/brotli_util.cc puffin/src/huffer.cc puffin/src/huffman_table.cc puffin/src/memory_stream.cc puffin/src/puff_reader.cc puffin/src/puff_writer.cc puffin/src/puffer.cc puffin/src/puffin_stream.cc puffin/src/puffpatch.cc puffin/src/puffin.pb.cc
PUFFPATCH_OBJ=$(addprefix build/,$(PUFFPATCH_SRC:.cc=.o))

puffin/src/puffpatch.cc: build/puffin/src/puffin.pb.cc
build/puffin/src/puffin.pb.cc: puffin/src/puffin.proto
	protoc -I=puffin/src/ --cpp_out=build/puffin/src/ $^

build/libpuffpatch.a: $(PUFFPATCH_OBJ)
	$(AR) rcs $@ $^

ZUCHIINI_SRC=zucchini/abs32_utils.cc zucchini/address_translator.cc zucchini/arm_utils.cc zucchini/binary_data_histogram.cc zucchini/buffer_sink.cc zucchini/buffer_source.cc zucchini/crc32.cc zucchini/disassembler.cc zucchini/disassembler_dex.cc zucchini/disassembler_elf.cc zucchini/disassembler_no_op.cc zucchini/disassembler_win32.cc zucchini/disassembler_ztf.cc zucchini/element_detection.cc zucchini/encoded_view.cc zucchini/ensemble_matcher.cc zucchini/equivalence_map.cc zucchini/heuristic_ensemble_matcher.cc zucchini/image_index.cc zucchini/imposed_ensemble_matcher.cc zucchini/io_utils.cc zucchini/patch_reader.cc zucchini/patch_writer.cc zucchini/reference_bytes_mixer.cc zucchini/reference_set.cc zucchini/rel32_finder.cc zucchini/rel32_utils.cc zucchini/reloc_elf.cc zucchini/reloc_win32.cc zucchini/target_pool.cc zucchini/targets_affinity.cc zucchini/zucchini_apply.cc zucchini/zucchini_gen.cc zucchini/zucchini_tools.cc
ZUCCHINI_OBJ=$(addprefix build/,$(ZUCHIINI_SRC:.cc=.o))

build/libzucchini.a: $(ZUCCHINI_OBJ)
	$(AR) rcs $@ $^
#CHROME_SRC=$(wildcard libchrome/base/third_party/*/*.cc libchrome/base/synchronization/*.cc libchrome/base/strings/*.cc libchrome/base/metrics/*.c libchrome/base/timer/*.cc libchrome/base/time/*.cc libchrome/base/process/*.cc libchrome/base/posix/*.cc libchrome/base/memory/*.cc libchrome/base/debug/*.cc libchrome/base/files/*.cc libchrome/base/*.cc )
CHROME_SRC=$(wildcard libchrome/base/third_party/*/*.cc libchrome/base/*/*.cc libchrome/base/*.cc)
CHROME_OBJ=$(addprefix build/,$(CHROME_SRC:.cc=.o))

build/libchrome.a: $(CHROME_OBJ)
	$(AR) rcs $@ $^

ALL_OBJ=build/
clean:
	rm -rf $(ALL_OBJ)
