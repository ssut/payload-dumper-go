# payload-dumper-go

An android OTA payload dumper written in Go.

## Features

![screenshot](https://i.imgur.com/IJtwoWU.png)

- Incredibly fast decompression. All decompression progresses are executed in parallel.
- Payload checksum verification.
- Support original zip package that contains payload.bin.

### Cautions

- Working on a SSD is highly recommended for performance reasons, a HDD could be a bottle-neck.

### Limitations

- Incremental OTA(delta) payload is not supported.

## Sources

https://android.googlesource.com/platform/system/update_engine/+/master/update_metadata.proto

## License

This source code is licensed under the Apache License 2.0 as described in the LICENSE file.
