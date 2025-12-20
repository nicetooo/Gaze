//go:build windows

package main

import _ "embed"

//go:embed bin/windows/adb.exe
var adbBinary []byte

//go:embed bin/windows/scrcpy.exe
var scrcpyBinary []byte
