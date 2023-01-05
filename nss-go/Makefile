CC := gcc
CFLAGS := -g -Wall -Wextra $(shell pkg-config --cflags glib-2.0)
LDFLAGS := $(shell pkg-config --libs glib-2.0)
SCRIPTPATH ?= /usr/libexec/aad-auth
DEB_HOST_GNU_TYPE ?= $(shell dpkg-architecture -qDEB_HOST_GNU_TYPE)

all: clean libnss_aad.so.2

libnss_aad.so.2:
	$(CC) -DSCRIPTPATH=\"$(SCRIPTPATH)\" *.c $(CFLAGS) $(LDFLAGS) -fPIC -shared -Wl,-soname,libnss_aad.so.2 -o libnss_aad.so.2

clean:
	rm -f libnss_aad.so.2

check:
	make clean
	codechecker check -b "make libnss_aad.so.2"

install: libnss_aad.so.2
	install -dm 0755 $(DESTDIR)/usr/lib/$(DEB_HOST_GNU_TYPE)
	install -m 0644 libnss_aad.so.2 $(DESTDIR)/usr/lib/$(DEB_HOST_GNU_TYPE)

.PHONY: all clean check install
