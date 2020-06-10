package timewheel

import (
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

const NN = 50000

func time1(N int) {
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			delay := time.Duration(rand.Intn(4000)+1000) * time.Millisecond
			tt := time.NewTimer(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
			t := time.NewTicker(delay)
			defer t.Stop()
			defer tt.Stop()
			for {
				ts := time.Now()
				<-t.C
				if time.Since(ts) > delay+500*time.Millisecond || time.Since(ts) < delay-500*time.Millisecond {
					fmt.Println(time.Since(ts))
				}
				//<-t.C
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

func time2(N int) {
	dt, _ := NewTimeWheelPool(64, 50*time.Millisecond, 200, TickSafeMode())
	dt.Start()
	defer dt.Stop()
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			tt := dt.Get().NewTimer(time.Duration(rand.Intn(4000)+1000) * time.Millisecond)
			defer tt.Stop()
			delay := time.Duration(rand.Intn(4000)+1000) * time.Millisecond
			t := dt.Get().NewTicker(delay)
			defer t.Stop()

			for {
				ts := time.Now()
				<-t.C
				if time.Since(ts) > delay+500*time.Millisecond || time.Since(ts) < delay-500*time.Millisecond {
					fmt.Println(time.Since(ts))
				}
				//	<-t.C
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

func time3() {
	go func() {
		res, _ := cpu.Percent(7*time.Second, false)
		fmt.Println(res)
	}()
	dt, _ := NewTimeWheelPool(64, 50*time.Millisecond, 1000, TickSafeMode())
	dt.Start()
	var wg sync.WaitGroup
	// for i := 0; i < N; i++ {
	wg.Add(1)
	go func() {
		t := dt.Get().NewTicker(time.Duration(500) * time.Millisecond)
		defer t.Stop()
		fmt.Println(time.Now())

		for {
			<-t.C
			fmt.Println(time.Now())
			// break
		}
		wg.Done()
	}()
	// }
	wg.Wait()

}

func Test_main(tt *testing.T) {
	go func() {
		fmt.Println(http.ListenAndServe(":9100", nil))
	}()
	for i := 16; i <= 16; i = i * 2 {

		N := i * NN
		t := time.Now()

		if true {
			go func() {
				res, _ := cpu.Percent(5*time.Second, false)
				fmt.Println("timer", N, ",go system cpu used", res)
			}()
			time1(N)
			fmt.Println("go system timer", N, ", cost", time.Since(t))
		} else {
			t = time.Now()
			go func() {
				res, _ := cpu.Percent(5*time.Second, false)
				fmt.Println("timer", N, ",timewheel cpu used", res)
			}()
			time2(N)
			fmt.Println("time wheel timer", N, ", cost", time.Since(t))
		}
	}

}
