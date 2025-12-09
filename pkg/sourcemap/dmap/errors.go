package dmap

import "errors"

var (
	// ErrInvalidMagic indicates the file does not start with "DMAP" magic bytes
	ErrInvalidMagic = errors.New("dmap: invalid magic bytes")

	// ErrUnsupportedVer indicates the file version is not supported
	ErrUnsupportedVer = errors.New("dmap: unsupported version")

	// ErrCorruptedFile indicates the file is corrupted or truncated
	ErrCorruptedFile = errors.New("dmap: file corrupted or truncated")

	// ErrOffsetOutOfRange indicates a byte offset is outside the source bounds
	ErrOffsetOutOfRange = errors.New("dmap: byte offset out of range")

	// ErrPositionNotFound indicates a position is not in any mapping
	ErrPositionNotFound = errors.New("dmap: position not in any mapping")
)
