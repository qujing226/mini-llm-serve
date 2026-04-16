package utils

import (
	"net"
	"strconv"

	"github.com/cockroachdb/errors"
)

func ExtractPortNumber(addr string) (int, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("invalid port")
	}
	return port, nil
}
