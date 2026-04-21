package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func GenerateUUIDv7() (string, error) {
	var u [16]byte

	ts := uint64(time.Now().UnixMilli())
	u[0] = byte(ts >> 40)
	u[1] = byte(ts >> 32)
	u[2] = byte(ts >> 24)
	u[3] = byte(ts >> 16)
	u[4] = byte(ts >> 8)
	u[5] = byte(ts)

	if _, err := rand.Read(u[6:]); err != nil {
		return "", err
	}

	u[6] = (u[6] & 0x0f) | 0x70 // version 7
	u[8] = (u[8] & 0x3f) | 0x80 // variant 10

	return formatUUID(u), nil
}

func MustGenerateUUIDv7() string {
	str, err := GenerateUUIDv7()
	if err != nil {
		return time.Now().Format("2006-01-02 15:04:05")
	}
	return str
}

func formatUUID(u [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])
	return string(buf[:])
}
