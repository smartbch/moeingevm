
all : sha256.o

%.o : %.c
	gcc -msse4.1 -msha -O3 -fPIC -c -I. -o $@ -c $<

clean:
	rm sha256.o
