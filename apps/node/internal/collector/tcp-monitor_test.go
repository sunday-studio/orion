package collector

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestRunTCPMonitorWithDialer(t *testing.T) {
	t.Run("connection succeeds", func(t *testing.T) {
		result := runTCPMonitorWithDialer(
			TCPMonitorConfig{Host: "127.0.0.1", Port: 5432, Timeout: time.Second},
			func(network string, address string, timeout time.Duration) (net.Conn, error) {
				if network != "tcp" {
					t.Fatalf("network = %q, want tcp", network)
				}
				if address != "127.0.0.1:5432" {
					t.Fatalf("address = %q, want 127.0.0.1:5432", address)
				}
				return nopConn{}, nil
			},
		)

		if result.Status != "up" {
			t.Fatalf("status = %q, want up", result.Status)
		}
		if result.Metrics["address"] != "127.0.0.1:5432" {
			t.Fatalf("address metric = %v", result.Metrics["address"])
		}
	})

	t.Run("connection fails", func(t *testing.T) {
		result := runTCPMonitorWithDialer(
			TCPMonitorConfig{Host: "127.0.0.1", Port: 5432, Timeout: time.Second},
			func(network string, address string, timeout time.Duration) (net.Conn, error) {
				return nil, errors.New("connection refused")
			},
		)

		if result.Status != "down" {
			t.Fatalf("status = %q, want down", result.Status)
		}
		if result.Error == nil || result.Error.Message != "connection refused" {
			t.Fatalf("error = %+v, want connection refused", result.Error)
		}
		if result.Metrics["failure_reason"] != "tcp connection failed" {
			t.Fatalf("failure_reason = %v, want tcp connection failed", result.Metrics["failure_reason"])
		}
	})
}

type nopConn struct{}

func (nopConn) Read([]byte) (int, error)         { return 0, nil }
func (nopConn) Write([]byte) (int, error)        { return 0, nil }
func (nopConn) Close() error                     { return nil }
func (nopConn) LocalAddr() net.Addr              { return nopAddr("local") }
func (nopConn) RemoteAddr() net.Addr             { return nopAddr("remote") }
func (nopConn) SetDeadline(time.Time) error      { return nil }
func (nopConn) SetReadDeadline(time.Time) error  { return nil }
func (nopConn) SetWriteDeadline(time.Time) error { return nil }

type nopAddr string

func (a nopAddr) Network() string { return string(a) }
func (a nopAddr) String() string  { return string(a) }
