OBJS = memzero.o ripemd160.o

all : ${OBJS}

%.o : %.c
	gcc -O3 -fPIC -c -o $@ -c $<

clean:
	rm *.o
