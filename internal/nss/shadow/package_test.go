//go:build !integrationtests

package shadow

// Expose setCacheOption for package tests
var SetCacheOption = setCacheOption
