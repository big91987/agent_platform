package platform

import "errors"

// Zero leaves execution unbounded; queued time and inter-turn waits never count.
func validateAgentExecutionLimits(a Agent) error {
	if a.TurnTimeoutSeconds < 0 || a.TurnTimeoutSeconds > 86400 {
		return errors.New("turn_timeout_seconds must be between 0 and 86400")
	}
	return nil
}
