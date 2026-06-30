//go:build windows

package ws

import v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"

func watchTerminalResize(_ v1.InteractiveSession) func() {
	return func() {}
}
