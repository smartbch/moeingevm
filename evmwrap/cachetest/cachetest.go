package main

/*
#cgo CXXFLAGS: -O3 -std=c++11
#cgo LDFLAGS: -lstdc++
#include "intcache.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seehuhn/mt19937"
)

const (
	Range          = int64(300)
	NumTestPerStep = int64(10)
	MinDuration    = int64(50)
	MaxDuration    = int64(150)
	NumGoRoutine   = int64(30)
)

var (
	TotalHit  = int64(0)
	TotalMiss = int64(0)
)

func add(key, value int64, height uint32) {
	C.add(C.int64_t(key), C.int64_t(value), C.uint32_t(height))
}

func give_back(key int64, height uint32) uint64 {
	sizes := C.give_back(C.int64_t(key), C.uint32_t(height))
	return uint64(sizes)
}

func borrow(key int64) *int64 {
	return (*int64)(C.borrow(C.int64_t(key)))
}

func check(jobID, num, duration, height int64) uint64 {
	negNumPtr := borrow(num)
	sizes := uint64(0)
	if atomic.LoadInt64(negNumPtr) == 0 { // cache miss
		time.Sleep(time.Duration(duration * int64(time.Millisecond)))
		add(num, -num, uint32(height))
		atomic.AddInt64(&TotalMiss, 1)
	} else { //cache hit
		if neg := atomic.LoadInt64(negNumPtr); neg != -num {
			panic(fmt.Sprintf("%d != %d", neg, -num))
		}
		time.Sleep(time.Duration(duration * int64(time.Millisecond)))
		if neg := atomic.LoadInt64(negNumPtr); neg != -num {
			panic(fmt.Sprintf("%d != %d", neg, -num))
		}
		sizes = give_back(num, uint32(height))
		atomic.AddInt64(&TotalHit, 1)
	}
	return sizes
}

func checkBetween(jobID, from, to, seed int64) {
	fmt.Printf("Starting Job#%d\n", jobID)
	rand := mt19937.New()
	rand.Seed(seed)
	for i := from; i < to; i++ {
		sizes := uint64(0)
		for j := int64(0); j < NumTestPerStep; j++ {
			offset := rand.Int63() % Range
			duration := MinDuration + rand.Int63()%(MaxDuration-MinDuration)
			sz := check(jobID, i-Range/2+offset, duration, i*NumTestPerStep+j)
			if sz != 0 {
				sizes = sz
			}
		}
		fmt.Printf("Job#%d %d h=%d m=%d s=%x\n", jobID, i, atomic.LoadInt64(&TotalHit), atomic.LoadInt64(&TotalMiss), sizes)
	}
}

func main() {
	seed := int64(10086)
	var wg sync.WaitGroup
	wg.Add(int(NumGoRoutine))
	for id := int64(0); id < NumGoRoutine; id++ {
		go func(id, seed int64) {
			checkBetween(id, 10000, 20000, seed+id)
			wg.Done()
		}(id, seed+id)
	}
	wg.Wait()
}
