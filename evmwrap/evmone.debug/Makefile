OBJS = \
   analysis.o \
   baseline.o \
   baseline_instruction_table.o \
   execution.o \
   instructions.o \
   instructions_calls.o \
   tracing.o \
   vm.o \

all : ${OBJS}

%.o : %.cpp
	g++ -O3 -static -std=c++17 -c -I ./include -I ../evmc/include/ -I ../intx/include/ -I ../keccak/include -o $@ -c $<

clean:
	rm *.o

