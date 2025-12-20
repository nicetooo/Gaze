//go:build darwin

package main

import _ "embed"

//go:embed bin/darwin/adb
var adbBinary []byte

//go:embed bin/darwin/scrcpy
var scrcpyBinary []byte

//go:embed bin/darwin/aapt
var aaptBinary []byte
