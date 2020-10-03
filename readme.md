# puddle

Puddle is a worker pool that was [forked from `workerpool`](https://github.com/gammazero/workerpool) and meant to help me learn channels and concurrency design.

The biggest difference is `workerpool` uses a [custom ring buffer implementation](https://github.com/gammazero/deque) while `puddle` uses the built-in `container/list` package.

---
\****not designed for production use***\*

---

### Install

```
go get -u github.com/oze4/puddle
```

### Usage 

```golang
numWorkers := 3
p := puddle.New(numWorkers) 

// Add job to our worker pool
p.Job(func() { 
  fmt.Printf("%s", "Hello")
  // ...
})

p.Job(func() {
  fmt.Printf("%s\n", ", World!")
  // ...
})

// You must always Seal the Puddle
// You CANNOT add jobs after Sealing the Puddle
p.Seal()
```