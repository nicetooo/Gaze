//go:build linux

package main

import _ "embed"

//go:embed bin/linux/adb
var adbBinary []byte

//go:embed bin/linux/scrcpy
var scrcpyBinary []byte
