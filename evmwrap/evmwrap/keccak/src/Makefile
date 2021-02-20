OBJS = keccak.o keccakf1600.o keccakf800.o

all : ${OBJS}

%.o : %.c
	gcc -O3 -c  -I ../include -o $@ -c $<

clean:
	rm *.o
