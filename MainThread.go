package giu

var callQueue chan func()

func checkRun() {
	if callQueue == nil {
		panic("mainthread: did not call Run")
	}
}

// transferTasks transfers tasks from global `callQueue` to `Run` -specific task queue.
// The function ends on readable event on `done` e.g. the channel is closed.
func transferTasks(to chan<- func(), done <-chan struct{}) {
	var (
		task  func()   // Current task to transfer or `nil`.
		tasks []func() // A local queue of tasks to transfer.
		// tasksCh is going to be assigned either `nil` or `to`. We use the fact that
		// `select` ignores `nil` channels. So we will assign `nil` here if there is
		// nothing to send, or `to` in case there's a task to be sent out.
		tasksCh chan<- func()
	)
	for {
		if task == nil && len(tasks) > 0 {
			// Pop next task from the task queue.
			task = tasks[0]
			copy(tasks[:], tasks[1:])
			tasks = tasks[:len(tasks)-1]
			// And setup the task channel for select.
			tasksCh = to
		}
		select {
		case f := <-callQueue:
			tasks = append(tasks, f)
		case tasksCh <- task: // nil-channels are ignored by select.
			task, tasksCh = nil, nil
		case <-done:
			return
		}
	}
}

// Run enables mainthread package functionality. To use mainthread package, put your main function
// code into the run function (the argument to Run) and simply call Run from the real main function.
//
// Run returns when run (argument) function finishes.
func Run(run func()) {
	// Note: Initializing global `callQueue`. This is potentially unsafe, as `callQueue` might
	// have been already initialized.
	// TODO(yarcat): Decide whether we should panic at this point or do something else.
	callQueue = make(chan func())

	tasks := make(chan func())
	done := make(chan struct{})
	go transferTasks(tasks, done)

	go func() {
		run()
		close(done) // `close` broadcasts it to all receivers.
	}()

	for {
		select {
		case f := <-tasks:
			f()
		case <-done:
			return
		}
	}
}

// CallNonBlock queues function f on the main thread and returns immediately. Does not wait until f
// finishes.
func CallNonBlock(f func()) {
	checkRun()
	callQueue <- f
}

// Call queues function f on the main thread and blocks until the function f finishes.
func Call(f func()) {
	checkRun()
	done := make(chan struct{})
	callQueue <- func() {
		f()
		done <- struct{}{}
	}
	<-done
}

// CallErr queues function f on the main thread and returns an error returned by f.
func CallErr(f func() error) error {
	checkRun()
	errChan := make(chan error)
	callQueue <- func() {
		errChan <- f()
	}
	return <-errChan
}

// CallVal queues function f on the main thread and returns a value returned by f.
func CallVal(f func() interface{}) interface{} {
	checkRun()
	respChan := make(chan interface{})
	callQueue <- func() {
		respChan <- f()
	}
	return <-respChan
}
