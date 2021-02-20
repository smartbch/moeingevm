
all : div.o

%.o : %.cpp
	g++ -O3 -fPIC -std=c++17 -c -I ../intx/include/ -o $@ -c $<

clean:
	rm div.o
