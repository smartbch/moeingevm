package main

/*
#cgo CXXFLAGS: -O3 -std=c++11
#cgo LDFLAGS: -lstdc++
#include "intcache.h"
*/
import "C"

import (
	"fmt"
	"time"
	"sync"
	"sync/atomic"

	"github.com/seehuhn/mt19937"
)

const (
	Range = int64(1000)
	NumTestPerStep = int64(100)
	MinDuration = int64(50)
	MaxDuration = int64(150)
	NumGoRoutine = int64(200)
)

var (
	TotalHit = int64(0)
	TotalMiss = int64(0)
)

func add(key, value int64, height uint32) {
	C.add(C.int64_t(key), C.int64_t(value), C.uint32_t(height));
}

func give_back(key int64, height uint32) {
	C.give_back(C.int64_t(key), C.uint32_t(height));
}

func borrow(key int64) (*int64) {
	return (*int64)(C.borrow(C.int64_t(key)));
}

func check(num, duration, height int64) {
	negNumPtr := borrow(num)
	if atomic.LoadInt64(negNumPtr) == 0 { // cache miss
		time.Sleep(time.Duration(duration*int64(time.Millisecond)))
		add(num, -num, uint32(height))
		atomic.AddInt64(&TotalMiss, 1)
	} else { //cache hit
		if neg := atomic.LoadInt64(negNumPtr); neg != -num {
			panic(fmt.Sprintf("%d != %d", neg, -num))
		}
		time.Sleep(time.Duration(duration*int64(time.Millisecond)))
		if neg := atomic.LoadInt64(negNumPtr); neg != -num {
			panic(fmt.Sprintf("%d != %d", neg, -num))
		}
		give_back(num, uint32(height))
		atomic.AddInt64(&TotalHit, 1)
	}
}

func checkBetween(jobID, from, to, seed int64) {
	fmt.Printf("Starting Job#%d\n", jobID)
	rand := mt19937.New()
	rand.Seed(seed)
	for i := from; i < to; i++ {
		fmt.Printf("Job#%d %d %d %d\n", jobID, i, atomic.LoadInt64(&TotalHit), atomic.LoadInt64(&TotalMiss))
		for j := int64(0); j < NumTestPerStep; j++ {
			offset := rand.Int63() % Range
			duration := MinDuration + rand.Int63() % (MaxDuration-MinDuration)
			check(i-Range/2+offset, duration, i*NumTestPerStep+j)
		}
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

