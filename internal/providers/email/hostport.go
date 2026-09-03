package email

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// splitHostPort splits a host value that may embed a port ("host:1143",
// "[::1]:1143") into a pure host and a port number; a bare host
// ("imap.example.com", "127.0.0.1", "::1", "[::1]") yields defaultPort.
// net.SplitHostPort semantics: only "missing port" and "too many colons"
// (a bare IPv6 literal) fall back to the default; an empty host after the
// split, a non-numeric port, or a port outside 1-65535 is an error.
func splitHostPort(value string, defaultPort int) (string, int, error) {
	host, portStr, err := net.SplitHostPort(value)
	if err != nil {
		var addrErr *net.AddrError
		if !errors.As(err, &addrErr) ||
			(addrErr.Err != "missing port in address" && addrErr.Err != "too many colons in address") {
			return "", 0, fmt.Errorf("invalid host %q: %v", value, err)
		}
		// Bare host, possibly a bracketed IPv6 literal like "[::1]":
		// keep the host and take the default port.
		host = value
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
		portStr = ""
	}
	if host == "" {
		return "", 0, fmt.Errorf("invalid host %q: no host before the port", value)
	}
	if portStr == "" {
		return host, defaultPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid host %q: port %q is not a number", value, portStr)
	}
	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid host %q: port must be between 1 and 65535, got %d", value, port)
	}
	return host, port, nil
}

// resolveDialServer defensively parses a STORED server config through
// splitHostPort so accounts persisted before host:port normalization
// (host "127.0.0.1:1143" with the port field left at the 993 default)
// still dial the embedded port. Precedence here is the reverse of
// account add: a port embedded in the stored host wins over the stored
// port field, because a legacy payload's stored port is just the default
// that never matched the embedded one. Normalized payloads carry a pure
// host, so splitHostPort falls through to the stored port unchanged.
func resolveDialServer(srv serverConfig, defaultPort int) (host string, port int, err error) {
	storedPort := srv.Port
	if storedPort == 0 {
		storedPort = defaultPort
	}
	return splitHostPort(srv.Host, storedPort)
}
