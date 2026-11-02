package ai

import "time"

func SetGatewayTimeout(d time.Duration) func() {
	previous := gatewayTimeout
	gatewayTimeout = d
	return func() { gatewayTimeout = previous }
}
