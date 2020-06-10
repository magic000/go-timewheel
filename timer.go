package timewheel

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	typeTimer taskType = iota
	typeTicker

	modeIsCircle  = true
	modeNotCircle = false

	modeIsAsync  = true
	modeNotAsync = false
)

type taskType int64
type taskID uint64

type Task struct {
	delay    time.Duration
	id       taskID
	index    int
	round    int
	callback func()

	async  bool
	stop   bool
	circle bool

	add bool
	// circleNum int
}

type optionCall func(*TimeWheel) error

func TickSafeMode() optionCall {
	return func(o *TimeWheel) error {
		o.tickQueue = make(chan time.Time, 10)
		return nil
	}
}

type TimeWheel struct {
	randomID uint64

	tick      time.Duration
	ticker    *time.Ticker
	tickQueue chan time.Time

	bucketsNum int
	buckets    []map[taskID]*Task // key: added item, value: *Task

	currentIndex int

	onceStart sync.Once

	addOrDelC chan *Task
	stopC     chan struct{}

	exited bool
}

// NewTimeWheel create new time wheel
func NewTimeWheel(tick time.Duration, bucketsNum int, options ...optionCall) (*TimeWheel, error) {
	if tick.Seconds() < 0.01 {
		return nil, errors.New("invalid params, must tick >= 10 ms")
	}
	if bucketsNum <= 0 {
		return nil, errors.New("invalid params, must bucketsNum > 0")
	}

	tw := &TimeWheel{
		// tick
		tick:      tick,
		tickQueue: make(chan time.Time, 10),

		// store
		bucketsNum:   bucketsNum,
		buckets:      make([]map[taskID]*Task, bucketsNum),
		currentIndex: 0,

		// signal
		addOrDelC: make(chan *Task, 1024*100),
		stopC:     make(chan struct{}),
	}

	for i := 0; i < bucketsNum; i++ {
		tw.buckets[i] = make(map[taskID]*Task, 1024)
	}

	for _, op := range options {
		op(tw)
	}

	return tw, nil
}

// Start start the time wheel
func (tw *TimeWheel) Start() {
	// onlye once start
	tw.onceStart.Do(
		func() {
			tw.ticker = time.NewTicker(tw.tick)
			go tw.schduler()
			go tw.tickGenerator()
		},
	)
}

func (tw *TimeWheel) tickGenerator() {
	if tw.tickQueue != nil {
		return
	}

	for !tw.exited {
		select {
		case <-tw.ticker.C:
			select {
			case tw.tickQueue <- time.Now():
			default:
				panic("raise long time blocking")
			}
		}
	}
}

func (tw *TimeWheel) schduler() {
	queue := tw.ticker.C
	if tw.tickQueue == nil {
		queue = tw.tickQueue
	}

	for {
		select {
		case <-queue:
			tw.handleTick()
		case task := <-tw.addOrDelC:
			if task.add {
				tw.store(task, false)
			} else {
				tw.remove(task)
			}
		case <-tw.stopC:
			tw.exited = true
			tw.ticker.Stop()
			return
		}
	}
}

// Stop stop the time wheel
func (tw *TimeWheel) Stop() {
	tw.stopC <- struct{}{}
}

func (tw *TimeWheel) handleTick() {
	bucket := tw.buckets[tw.currentIndex]
	for k, task := range bucket {
		if task.stop {
			delete(tw.buckets[task.index], task.id)
			continue
		}

		if bucket[k].round > 0 {
			bucket[k].round--
			continue
		}

		if task.async {
			go task.callback()
		} else {
			// optimize gopool
			task.callback()
		}

		// circle
		if task.circle {
			delete(tw.buckets[task.index], task.id)
			tw.store(task, true)
			continue
		}

		// gc
		delete(tw.buckets[task.index], task.id)
	}

	if tw.currentIndex == tw.bucketsNum-1 {
		tw.currentIndex = 0
		return
	}

	tw.currentIndex++
}

func (tw *TimeWheel) addAny(delay time.Duration, callback func(), circle, async bool) *Task {
	if delay < tw.tick {
		delay = tw.tick / 2
	}

	id := tw.genUniqueID()

	var task *Task
	task = new(Task)

	task.add = true
	task.delay = delay
	task.id = id
	task.stop = false
	task.callback = callback
	task.circle = circle
	task.async = async // refer to src/runtime/time.go

	tw.addOrDelC <- task
	return task
}

func (tw *TimeWheel) store(task *Task, circle bool) {
	task.round, task.index = tw.calculateRoundIndex(task.delay, circle)
	tw.buckets[task.index][task.id] = task
}

func (tw *TimeWheel) calculateRoundIndex(delay time.Duration, circle bool) (round int, index int) {
	delaySeconds := delay.Seconds()
	tickSeconds := tw.tick.Seconds()
	currentIndex := tw.currentIndex
	if circle {
		currentIndex++
	}
	round = int((delaySeconds / tickSeconds) / float64(tw.bucketsNum))
	index = (int(float64(currentIndex) + delaySeconds/tickSeconds)) % tw.bucketsNum
	return
}

func (tw *TimeWheel) Remove(task *Task) error {
	task.add = false
	tw.addOrDelC <- task
	return nil
}

func (tw *TimeWheel) remove(task *Task) {
	delete(tw.buckets[task.index], task.id)
}

func (tw *TimeWheel) NewTimer(delay time.Duration) *Timer {
	queue := make(chan bool, 1) // buf = 1, refer to src/time/sleep.go
	task := tw.addAny(delay,
		func() {
			notfiyChannel(queue)
		},
		modeNotCircle,
		modeNotAsync,
	)

	// init timer
	timer := &Timer{
		tw:   tw,
		C:    queue, // faster
		task: task,
	}

	return timer
}

func (tw *TimeWheel) AfterFunc(delay time.Duration, callback func()) *Timer {
	queue := make(chan bool, 1)
	task := tw.addAny(delay,
		func() {
			callback()
			notfiyChannel(queue)
		},
		modeNotCircle, modeNotAsync,
	)

	timer := &Timer{
		tw:   tw,
		C:    queue, // faster
		task: task,
		fn:   callback,
	}

	return timer
}

func (tw *TimeWheel) NewTickerFunc(delay time.Duration, callback func()) *Ticker {
	queue := make(chan bool, 1)
	task := tw.addAny(delay,
		func() {
			callback()
			notfiyChannel(queue)
		},
		modeIsCircle,
		modeNotAsync,
	)

	// init ticker
	ticker := &Ticker{
		task: task,
		tw:   tw,
		C:    queue,
	}

	return ticker
}

func (tw *TimeWheel) NewTicker(delay time.Duration) *Ticker {
	queue := make(chan bool, 1)
	task := tw.addAny(delay,
		func() {
			notfiyChannel(queue)
		},
		modeIsCircle,
		modeNotAsync,
	)

	// init ticker
	ticker := &Ticker{
		task: task,
		tw:   tw,
		C:    queue,
	}

	return ticker
}

func (tw *TimeWheel) After(delay time.Duration) <-chan time.Time {
	queue := make(chan time.Time, 1)
	tw.addAny(delay,
		func() {
			queue <- time.Now()
		},
		modeNotCircle, modeNotAsync,
	)
	return queue
}

func (tw *TimeWheel) Sleep(delay time.Duration) {
	queue := make(chan bool, 1)
	tw.addAny(delay,
		func() {
			queue <- true
		},
		modeNotCircle, modeNotAsync,
	)
	<-queue
}

// similar to golang std timer
type Timer struct {
	task *Task
	tw   *TimeWheel
	fn   func() // external custom func
	C    chan bool
}

func (t *Timer) Reset(delay time.Duration) {
	var task *Task
	async := t.task.async
	circle := t.task.circle

	t.task.stop = true
	t.tw.Remove(t.task)

	if t.fn != nil { // use AfterFunc
		task = t.tw.addAny(delay,
			func() {
				t.fn()
				notfiyChannel(t.C)
			},
			circle, async, // must async mode
		)
	} else {
		task = t.tw.addAny(delay,
			func() {
				notfiyChannel(t.C)
			},
			circle, async)
	}

	t.task = task
}

func (t *Timer) Stop() {
	t.task.stop = true
	t.tw.Remove(t.task)
}

func (t *Timer) StopFunc(callback func()) {
	t.fn = callback
}

type Ticker struct {
	tw   *TimeWheel
	task *Task

	C chan bool
}

func (t *Ticker) Stop() {
	t.task.stop = true
	t.tw.Remove(t.task)
}

func notfiyChannel(q chan bool) {
	select {
	case q <- true:
	default:
	}
}

func (tw *TimeWheel) genUniqueID() taskID {
	id := atomic.AddUint64(&tw.randomID, 1)
	return taskID(id)
}
