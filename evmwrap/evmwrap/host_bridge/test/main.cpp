#include <iostream>
#include <vector>
#include <string>
#include "../host_context.h"

using namespace std;

class runnable {
	static vector<runnable*> instances;
	string name;
public:
	runnable(const string& n): name(n) {
		runnable::instances.push_back(this);
	}
	static void run_all() {
		for(auto inst : runnable::instances) {
			inst->run();
		}
	}
	virtual void run() = 0;
};

vector<runnable*> runnable::instances;

#define RUN(name) struct name : runnable { name() : runnable("name") {} void run() {
#define TOKEN_PASTE(x, y) x##y
#define CAT(x,y) TOKEN_PASTE(x,y)
#define END } } CAT(i_, __LINE__);

// ==========================================================================

char hex_char(uint8_t i) {
	if(i <= 9) return '0'+i;
	return 'A'+i-10;
}

string to_hex(uint8_t* data, size_t size) {
	vector<char> vec(2*size);
	for(size_t i = 0; i < size; i++) {
		vec[2*i] = hex_char((data[i]>>4)&0xF);
		vec[2*i+1] = hex_char(data[i]&0xF);
	}
	return string(vec.data(), vec.size());
}

RUN(run1)
	evmc_address creater = {.bytes={0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19}};
	cout<<"creater: "<<to_hex(&creater.bytes[0], 20)<<endl;
	evmc_address created_addr = create_contract_addr(creater, uint64_t(0x0102030405060708L));
	string imp_addr = to_hex(&created_addr.bytes[0], 20);
	string ref_addr("FDB30186ECDAB7C6B9C265A192318137EFD02316");
	cout<<"created_addr: "<<imp_addr<<endl;
	assert(imp_addr == ref_addr);

	evmc_address creater2 = {.bytes={1,1,3,3,5,5,7,7,9,9,11,11,13,13,15,15,17,17,19,19}};
	created_addr = create_contract_addr(creater2, uint64_t(0x0202040406060808L));
	imp_addr = to_hex(&created_addr.bytes[0], 20);
	ref_addr = "159E8779D35529F8EC6F26D573D97483273632CF";
	cout<<"created_addr: "<<imp_addr<<endl;
	assert(imp_addr == ref_addr);

	evmc_bytes32 salt1, hash1, salt2, hash2;
	for(size_t i = 0; i < 32; i++) {
		salt1.bytes[i] = 1;
		hash1.bytes[i] = 11;
		salt2.bytes[i] = 2;
		hash2.bytes[i] = 22;
	}
	created_addr = create2_contract_addr(creater, salt1, hash1);
	imp_addr = to_hex(&created_addr.bytes[0], 20);
	ref_addr = "53507E2054B21D1DE76CCD8325F765F7258306FE";
	cout<<"created_addr: "<<imp_addr<<endl;
	assert(imp_addr == ref_addr);

	created_addr = create2_contract_addr(creater2, salt2, hash2);
	imp_addr = to_hex(&created_addr.bytes[0], 20);
	ref_addr = "0BE5C80CA61454071274B0865DCB8C6F72A5D384";
	cout<<"created_addr: "<<imp_addr<<endl;
	assert(imp_addr == ref_addr);

END

int main() {
	runnable::run_all();
	return 0;
}
