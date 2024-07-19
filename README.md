# payload-dumper-go

An android OTA payload dumper written in Go.

## Features

![screenshot](https://i.imgur.com/IJtwoWU.png)

See how fast payload-dumper-go is: https://imgur.com/a/X6HKJT4. (MacBook Pro 16-inch 2019 i9-9750H, 16G)

- Incredibly fast decompression. All decompression progresses are executed in parallel.
- Payload checksum verification.
- Support original zip package that contains payload.bin.

### Cautions

- There's just one dependency you need to install on your system: `xz`. (The reason I didn't use the pure Go implementation is written in the [Performance](#performance) section below.)
- Working on a SSD is highly recommended for performance reasons, a HDD could be a bottle-neck.

### Limitations

- Incremental OTA (delta) payload is not supported yet. ([#44](https://github.com/ssut/payload-dumper-go/pull/44))

## Installation

### Linux and macOS (From releases, recommended)

1. Download the latest binary for your platform from [here](https://github.com/ssut/payload-dumper-go/releases) and extract the contents of the downloaded file to a directory on your system.
2. Make sure the extracted binary file has executable permissions. You can use the following command to set the permissions if necessary:

```
chmod +x payload-dumper-go
```

3. Run the following command to add the directory path to your system's PATH environment variable:

```
export PATH=$PATH:/path/to/payload-dumper-go
```

Note: This command sets the PATH environment variable only for the current terminal session. To make it permanent, you need to add the command to your system's profile file (e.g. .bashrc or .zshrc for Linux/Unix systems).

### macOS (Homebrew)

Just simply run:

```sh
$ brew install payload-dumper-go
```

### Windows

1. Download the latest binary for your platform from [here](https://github.com/ssut/payload-dumper-go/releases) and extract the contents of the downloaded file to a directory on your system.
2. Open the Start menu and search for "Environment Variables".
3. Click on "Edit the system environment variables".
4. Click on the "Environment Variables" button at the bottom right corner of the "System Properties" window.
5. Under "System Variables", scroll down and click on the "Path" variable, then click on "Edit".
6. Click "New" and add the path to the directory where the extracted binary is located.
7. Click "OK" on all the windows to save the changes.

## Usage

Run the following command in your terminal:

```
payload-dumper-go /path/to/payload.bin
```

## Performance

Machine: MacBook Pro 16-inch 2021 (Apple M1 Max, 64G), OS: macOS Sonoma 14.5, Go: 1.22.4.

Tested with a 2.31GB payload.bin file from https://developers.google.com/android/ota (akita).

```shell
payload.bin: payload.bin
Payload Version: 2
Payload Manifest Length: 154250
Payload Manifest Signature Length: 523
Found partitions:
abl (1.8 MB), bl1 (16 kB), bl2 (537 kB), bl31 (106 kB), boot (67 MB), dtbo (17 MB), gcf (8.2 kB), gsa (348 kB), gsa_bl1 (33 kB), init_boot (8.4 MB), ldfw (2.4 MB), modem (102 MB), pbl (49 kB), product (3.4 GB), pvmfw (1.0 MB), system (821 MB), system_dlkm (11 MB), system_ext (288 MB), tzsw (7.9 MB), vbmeta (12 kB), vbmeta_system (8.2 kB), vbmeta_vendor (4.1 kB), vendor (693 MB), vendor_boot (67 MB), vendor_dlkm (28 MB), vendor_kernel_boot (67 MB)
Number of workers: 4
abl (1.8 MB)                [===================================================================================================================] 100 %
bl2 (537 kB)                [===================================================================================================================] 100 %
bl1 (16 kB)                 [===================================================================================================================] 100 %
bl31 (106 kB)               [===================================================================================================================] 100 %
boot (67 MB)                [===================================================================================================================] 100 %
dtbo (17 MB)                [===================================================================================================================] 100 %
gcf (8.2 kB)                [===================================================================================================================] 100 %
gsa (348 kB)                [===================================================================================================================] 100 %
gsa_bl1 (33 kB)             [===================================================================================================================] 100 %
init_boot (8.4 MB)          [===================================================================================================================] 100 %
ldfw (2.4 MB)               [===================================================================================================================] 100 %
modem (102 MB)              [===================================================================================================================] 100 %
pbl (49 kB)                 [===================================================================================================================] 100 %
product (3.4 GB)            [===================================================================================================================] 100 %
pvmfw (1.0 MB)              [===================================================================================================================] 100 %
system (821 MB)             [===================================================================================================================] 100 %
system_dlkm (11 MB)         [===================================================================================================================] 100 %
system_ext (288 MB)         [===================================================================================================================] 100 %
tzsw (7.9 MB)               [===================================================================================================================] 100 %
vbmeta (12 kB)              [===================================================================================================================] 100 %
vbmeta_system (8.2 kB)      [===================================================================================================================] 100 %
vbmeta_vendor (4.1 kB)      [===================================================================================================================] 100 %
vendor (693 MB)             [===================================================================================================================] 100 %
vendor_boot (67 MB)         [===================================================================================================================] 100 %
vendor_dlkm (28 MB)         [===================================================================================================================] 100 %
vendor_kernel_boot (67 MB)  [===================================================================================================================] 100 %
go run *.go payload.bin  87.93s user 3.51s system 145% cpu 1:02.99 total
```

### Why not use the pure Go implementation for xz decompression?

[The pure Go implementation of xz](https://github.com/ulikunitz/xz) is very slow compared to [the C implementation used with CGO](https://github.com/spencercw/go-xz). Here's the result with the same payload.bin file on the same conditions:

```shell
payload.bin: payload.bin
Payload Version: 2
Payload Manifest Length: 154250
Payload Manifest Signature Length: 523
Found partitions:
abl (1.8 MB), bl1 (16 kB), bl2 (537 kB), bl31 (106 kB), boot (67 MB), dtbo (17 MB), gcf (8.2 kB), gsa (348 kB), gsa_bl1 (33 kB), init_boot (8.4 MB), ldfw (2.4 MB), modem (102 MB), pbl (49 kB), product (3.4 GB), pvmfw (1.0 MB), system (821 MB), system_dlkm (11 MB), system_ext (288 MB), tzsw (7.9 MB), vbmeta (12 kB), vbmeta_system (8.2 kB), vbmeta_vendor (4.1 kB), vendor (693 MB), vendor_boot (67 MB), vendor_dlkm (28 MB), vendor_kernel_boot (67 MB)
Number of workers: 4
abl (1.8 MB)                [===================================================================================================================] 100 %
bl1 (16 kB)                 [===================================================================================================================] 100 %
bl2 (537 kB)                [===================================================================================================================] 100 %
bl31 (106 kB)               [===================================================================================================================] 100 %
boot (67 MB)                [===================================================================================================================] 100 %
dtbo (17 MB)                [===================================================================================================================] 100 %
gcf (8.2 kB)                [===================================================================================================================] 100 %
gsa (348 kB)                [===================================================================================================================] 100 %
gsa_bl1 (33 kB)             [===================================================================================================================] 100 %
init_boot (8.4 MB)          [===================================================================================================================] 100 %
ldfw (2.4 MB)               [===================================================================================================================] 100 %
modem (102 MB)              [===================================================================================================================] 100 %
pbl (49 kB)                 [===================================================================================================================] 100 %
product (3.4 GB)            [===================================================================================================================] 100 %
pvmfw (1.0 MB)              [===================================================================================================================] 100 %
system (821 MB)             [===================================================================================================================] 100 %
system_dlkm (11 MB)         [===================================================================================================================] 100 %
system_ext (288 MB)         [===================================================================================================================] 100 %
tzsw (7.9 MB)               [===================================================================================================================] 100 %
vbmeta (12 kB)              [===================================================================================================================] 100 %
vbmeta_system (8.2 kB)      [===================================================================================================================] 100 %
vbmeta_vendor (4.1 kB)      [===================================================================================================================] 100 %
vendor (693 MB)             [===================================================================================================================] 100 %
vendor_boot (67 MB)         [===================================================================================================================] 100 %
vendor_dlkm (28 MB)         [===================================================================================================================] 100 %
vendor_kernel_boot (67 MB)  [===================================================================================================================] 100 %
go run *.go payload.bin  587.89s user 2428.69s system 248% cpu 20:12.19 total
```

As you can see, the pure Go implementation is about 6~ times slower than the C implementation.

## Sources

https://android.googlesource.com/platform/system/update_engine/+/master/update_metadata.proto

## License

This source code is licensed under the Apache License 2.0 as described in the LICENSE file.
