# payload-dumper-go

An android OTA payload dumper written in Go.

## Features

![screenshot](https://i.imgur.com/IJtwoWU.png)

See how fast payload-dumper-go is: https://imgur.com/a/X6HKJT4. (MacBook Pro 16-inch 2019 i9-9750H, 16G)

- Incredibly fast decompression. All decompression progresses are executed in parallel.
- Payload checksum verification.
- Support original zip package that contains payload.bin.

### Cautions

- Working on a SSD is highly recommended for performance reasons, a HDD could be a bottle-neck.

### Limitations

- Incremental OTA (delta) payload is not supported.

## Installation

0. Download the latest binary for your platform from [here](https://github.com/ssut/payload-dumper-go/releases) and extract the contents of the downloaded file to a directory on your system.

### Linux and OSX

1. Make sure the extracted binary file has executable permissions. You can use the following command to set the permissions if necessary:
```
chmod +x payload-dumper-go
```
2. Run the following command to add the directory path to your system's PATH environment variable:
```
export PATH=$PATH:/path/to/payload-dumper-go
```
Note: This command sets the PATH environment variable only for the current terminal session. To make it permanent, you need to add the command to your system's profile file (e.g. .bashrc or .zshrc for Linux/Unix systems).

### Windows

1. Open the Start menu and search for "Environment Variables".
2. Click on "Edit the system environment variables".
3. Click on the "Environment Variables" button at the bottom right corner of the "System Properties" window.
4. Under "System Variables", scroll down and click on the "Path" variable, then click on "Edit".
5. Click "New" and add the path to the directory where the extracted binary is located.
6. Click "OK" on all the windows to save the changes.

## Usage

Run the following command in your terminal:
```
payload-dumper-go /path/to/payload.bin
```

## Sources

https://android.googlesource.com/platform/system/update_engine/+/master/update_metadata.proto

## License

This source code is licensed under the Apache License 2.0 as described in the LICENSE file.
