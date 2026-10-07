//go:build !windows

package main

// runAsService is a no-op outside Windows; systemd runs the binary directly.
func runAsService(cfgPath string) (bool, error) { return false, nil }
