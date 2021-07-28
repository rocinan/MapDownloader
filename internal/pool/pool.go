package pool

type Job func()

type Worker struct {
	WorkerPool chan chan Job
	JobChannel chan Job
	quit       chan bool
}

func NewWorker(pool chan chan Job) *Worker {
	return &Worker{
		WorkerPool: pool,
		JobChannel: make(chan Job),
		quit:       make(chan bool),
	}
}

func (w *Worker) Start() {
	go func() {
		for {
			w.WorkerPool <- w.JobChannel
			select {
			case job := <-w.JobChannel:
				job()
			case <-w.quit:
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	w.quit <- true
}

type Dispatcher struct {
	WorkerCap  int
	JobQueue   chan Job
	WorkerPool chan chan Job
}

func NewDispatcher(maxWorkers int, maxQueue int) *Dispatcher {
	return &Dispatcher{maxWorkers, make(chan Job, maxQueue), make(chan chan Job, maxWorkers)}
}

func (d *Dispatcher) Run() {
	for i := 0; i <= d.WorkerCap; i++ {
		worker := NewWorker(d.WorkerPool)
		worker.Start()
	}
	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for {
		select {
		case job := <-d.JobQueue:
			jobChannel := <-d.WorkerPool
			jobChannel <- job
		}
	}
}
