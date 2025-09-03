// Package config provides error definitions for configuration management
package config

import "errors"

// Configuration validation errors
var (
	ErrInvalidAppName        = errors.New("invalid application name")
	ErrInvalidEnvironment    = errors.New("invalid environment")
	ErrInvalidLogLevel       = errors.New("invalid log level")
	ErrInvalidPort           = errors.New("invalid port number")
	ErrInvalidMaxConnections = errors.New("invalid max connections")
	ErrInvalidMaxActors      = errors.New("invalid max actors")
	ErrInvalidMailboxSize    = errors.New("invalid mailbox size")
)

// Configuration loading errors
var (
	ErrConfigFileNotFound  = errors.New("configuration file not found")
	ErrConfigParseError    = errors.New("configuration parse error")
	ErrConfigValidateError = errors.New("configuration validation error")
	ErrEnvironmentVarError = errors.New("environment variable error")
	ErrConfigWatchError    = errors.New("configuration watch error")
)

// Configuration provider errors
var (
	ErrProviderNotSupported = errors.New("configuration provider not supported")
	ErrProviderConnection   = errors.New("configuration provider connection error")
	ErrProviderTimeout      = errors.New("configuration provider timeout")
)
