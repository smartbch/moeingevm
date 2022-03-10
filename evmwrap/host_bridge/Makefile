OBJS = host_context.o tx_ctrl.o

all : libevmwrap.a

libevmwrap.a : ${OBJS}
	ar rvs libevmwrap.a ${OBJS} ../keccak/src/keccak.o ../evmone/analysis.o ../evmone/baseline.o ../evmone/baseline_instruction_table.o ../evmone/execution.o ../evmone/instructions.o ../evmone/instructions_calls.o ../evmone/tracing.o ../evmone/vm.o ../evmc/hex/hex.o ../evmc/instructions/instruction_metrics.o  ../evmc/instructions/instruction_names.o ../sha256/sha256.o ../ripemd160/ripemd160.o ../ripemd160/memzero.o
 
%.o : %.cpp
	g++ -Wall -O3 -static -std=c++17 -c -I ../evmone/include/ -I ../evmc/include/ -I ../intx/include/ -I ../keccak/include -o $@ -c $<

clean:
	rm *.o *.a
