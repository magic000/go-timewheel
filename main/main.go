package main

import (
	"fmt"
	"math/rand"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"code.byted.org/videoarch/timewheel"
)

const N = 50000

func time1() {
	go func() {
		res, _ := cpu.Percent(7*time.Second, false)
		fmt.Println(res)
	}()
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			tt := time.NewTimer(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
			t := time.NewTicker(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
			defer t.Stop()
			defer tt.Stop()
			for {
				<-t.C
				// <-t.C
				// <-t.C
				<-tt.C
				// tt.Reset(time.Duration(3000) * time.Millisecond)
				// tt.Reset(time.Duration(3000) * time.Millisecond)

				break
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func time2() {
	go func() {
		res, _ := cpu.Percent(7*time.Second, false)
		fmt.Println(res)
	}()
	dt, _ := timewheel.NewTimeWheelPool(64, 50*time.Millisecond, 100, timewheel.TickSafeMode())
	dt.Start()
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			tt := dt.Get().NewTimer(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
			t := dt.Get().NewTicker(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
			defer t.Stop()

			defer tt.Stop()
			for {
				<-t.C
				// <-t.C
				// <-t.C
				<-tt.C
				// tt.Reset(time.Duration(3000) * time.Millisecond)
				// tt.Reset(time.Duration(3000) * time.Millisecond)

				break
			}
			wg.Done()
		}()
	}
	wg.Wait()

}

func main() {
	// go log.Fatal(http.ListenAndServe(":8080", nil))

	t := time.Now()
	time1()
	fmt.Println("time1 const", time.Since(t))
	t = time.Now()
	time2()

	// tt := timewheel.DefaultTimeWheel.NewTicker(time.Duration(12000)*time.Millisecond + 1)
	// for {
	// 	<-tt.C
	// 	fmt.Println(time.Now())
	// }

	fmt.Println("time2 const", time.Since(t))
	// select {}
}
