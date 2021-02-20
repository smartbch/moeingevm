OBJS = \
   analysis.o \
   evmone.o \
   execution.o \
   instructions.o \

all : ${OBJS}

%.o : %.cpp
	g++ -O3 -fPIC -std=c++17 -c -I ./include -I ../evmc/include/ -I ../intx/include/ -I ../keccak/include -o $@ -c $<

clean:
	rm *.o
