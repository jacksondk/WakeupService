package wol

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// Send broadcasts a Wake-on-LAN magic packet for the given MAC address.
// broadcastAddr should be e.g. "255.255.255.255:9".
func Send(macAddr, broadcastAddr string) error {
	mac, err := parseMac(macAddr)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %w", err)
	}

	packet := buildPacket(mac)

	conn, err := net.Dial("udp", broadcastAddr)
	if err != nil {
		return fmt.Errorf("failed to open UDP connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send magic packet: %w", err)
	}
	return nil
}

// parseMac parses a MAC address string (colon or dash separated) into 6 bytes.
func parseMac(macAddr string) ([]byte, error) {
	normalized := strings.ReplaceAll(macAddr, ":", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	if len(normalized) != 12 {
		return nil, fmt.Errorf("expected 12 hex chars, got %d", len(normalized))
	}
	return hex.DecodeString(normalized)
}

// buildPacket constructs the 102-byte WoL magic packet:
// 6 bytes of 0xFF followed by the target MAC repeated 16 times.
func buildPacket(mac []byte) []byte {
	packet := make([]byte, 6+16*6)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}
	return packet
}
