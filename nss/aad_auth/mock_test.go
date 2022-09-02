package main

import "fmt"

const defaultEntry string = "myuser%d@domain.com:x:%d:%d::/home/myuser%d@domain.com:/bin/bash"

type entryMock struct {
	str string
}

func (e entryMock) String() string {
	return e.str
}

func newMockEntries(n int) []fmt.Stringer {
	var mockEntries []fmt.Stringer
	for i := 1; i < n+1; i++ {
		e := entryMock{str: fmt.Sprintf(defaultEntry, i, i, i, i)}
		mockEntries = append(mockEntries, e)
	}
	return mockEntries
}
