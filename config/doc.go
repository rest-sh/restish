// Package config is the public API for reading and writing Restish config.
//
// The supported surface is the on-disk restish.json schema structs, path
// discovery, strict JSONC loading/parsing, whole-file saving, validation, and
// small helpers that interpret config field values such as operation bases and
// URL overrides. Comment-preserving mutation helpers, v1 migration internals,
// and file-lock/atomic-write primitives live under internal packages so they
// can evolve with the CLI.
package config
