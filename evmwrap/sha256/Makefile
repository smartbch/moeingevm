
all : sha256.o

%.o : %.c
	gcc -O3 -fPIC -c -I. -o $@ -c $<

clean:
	rm sha256.o
