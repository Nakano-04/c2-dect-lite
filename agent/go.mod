module c2-dect-lite/agent

go 1.24.0

toolchain go1.24.4

require (
	c2-dect-lite/scripttrust v0.0.0
	github.com/yuin/gopher-lua v1.1.1
	golang.org/x/crypto v0.46.0
)

replace c2-dect-lite/scripttrust => ../scripttrust
