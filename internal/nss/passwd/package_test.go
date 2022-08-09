//go:build !integrationtests

package passwd

// Expose setCacheOption for package tests.
var SetCacheOption = setCacheOption
