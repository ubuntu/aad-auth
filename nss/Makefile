CC=gcc
CFLAGS=-g -Wall -Wextra $(shell pkg-config --cflags glib-2.0)
LDFLAGS=$(shell pkg-config --libs glib-2.0)

all: clean libnss_aad.so.2 copy

libnss_aad.so.2:
	$(CC) -DSCRIPTPATH=\"/home/u/getent.sh\" passwd.c group.c shadow.c common.c  $(CFLAGS) $(LDFLAGS) -fPIC -shared -Wl,-soname,libnss_aad.so.2 -o libnss_aad.so.2

clean:
	rm -f libnss_aad.so.2

check:
	make clean
	codechecker check -b "make libnss_aad.so.2"

copy:
	scp libnss_aad.so.2 u@192.168.122.144:~/
	scp getent.sh u@192.168.122.144:~/

