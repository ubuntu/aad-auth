package main

import "testing"

func TestGetpwnamR(t *testing.T) {

	// Load cache with env variables to point to copy of testdata

	// We build the .so
	// We have tags to build with or without AAD mock. The mock can be controlled with environment variables

	// LD_LIBRARY_PATH=*.so ->
	// LD_PRELOAD=./libnss_aad.so.2 getent passwd u6@uaadtest.onmicrosoft.com

}
