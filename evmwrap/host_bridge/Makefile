OBJS = host_context.o tx_ctrl.o

all : libevmwrap.so

libevmwrap.so : ${OBJS}
	g++ -shared -o libevmwrap.so ${OBJS} ../keccak/src/keccak.o ../evmone/analysis.o ../evmone/execution.o ../evmone/evmone.o ../evmone/instructions.o ../sha256/sha256.o ../ripemd160/ripemd160.o ../ripemd160/memzero.o && cp libevmwrap.so ../evmtests/
 
%.o : %.cpp
	g++ -Wall -O3 -fPIC -std=c++17 -c -I ../evmone/include/ -I ../evmc/include/ -I ../intx/include/ -I ../keccak/include -o $@ -c $<

clean:
	rm *.o *.so
