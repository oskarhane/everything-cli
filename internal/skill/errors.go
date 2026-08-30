package skill

import "errors"

// ErrNoAgentsDetected is returned by Install when no agentFilter is supplied
// and no supported agent is detected on the host filesystem.
var ErrNoAgentsDetected = errors.New("skill: no supported agents detected")

// ErrUnknownAgent is returned by Install / Remove when agentFilter does not
// match any entry in the AGENTS catalog (case-insensitive).
var ErrUnknownAgent = errors.New("skill: unknown agent")

// ErrAgentNotDetected is returned by Install when agentFilter matches a
// known agent but its DetectDir is missing on the host filesystem.
var ErrAgentNotDetected = errors.New("skill: agent not detected on host")
