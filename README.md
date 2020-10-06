# payload-dumper-go

An android OTA payload dumper written in Go.

Download prebuilt binaries for macOS and windows via [here](https://github.com/ssut/payload-dumper-go/releases).

## Features

![screenshot](https://i.imgur.com/IJtwoWU.png)

See how fast payload-dumper-go is: https://imgur.com/a/X6HKJT4. (MacBook Pro 16-inch 2019 i9-9750H, 16G)

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
