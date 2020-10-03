package puddle

import 	"container/list"

// List wraps `container/list`
type List struct {
	list.List
}

// PopFront removes then returns first element in list as func()
func (l *List) PopFront() func() {
	f := l.Front()
	l.Remove(f)
	return f.Value.(func())
}