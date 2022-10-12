OBJS = hex.o

all : ${OBJS}

%.o : %.cpp
	g++ -O3 -c -fPIC -std=c++17 -I ../include -o $@ -c $<

clean:
	rm *.o
