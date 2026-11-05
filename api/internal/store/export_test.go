package store

// The queries plans_test.go checks, for the tests outside the package.
var (
	DeleteExpiredLoginCodes  = deleteExpiredLoginCodes
	DeleteExpiredSessions    = deleteExpiredSessions
	DeleteEndedLoginFailures = deleteEndedLoginFailures
	DeleteStaleKnownDevices  = deleteStaleKnownDevices
)
