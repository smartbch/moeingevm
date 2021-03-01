package ebp

//#include "bridge.h"
import "C"

func init() {
	res := int(C.init_dl())
	if res == C.ENV_NOT_DEFINED {
		panic("Environment variable EVMWRAP is not defined")
	} else if res == C.FAIL_TO_OPEN {
		panic("Cannot open the dynamic library specified by EVMWRAP")
	} else if res == C.SYMBOL_NOT_FOUND {
		panic("Cannot find zero_depth_call function in dynamic library")
	}
}
