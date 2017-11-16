// Packge ilist provides the implementation of intrusive linked lists

package ilist

// Linker is the interface that objects must implement if they want to be added
// to and/or removed from List objects
//
// N.B. When substituted in a template instantiation, Linker doesn't need to
// be an interface, and in most cases won't be
type Linker interface {
	Next() 	Linker
	Prev()	Linker
	SetNext(Linker)
	SetPrev(Linker)
}

// List is an intrusive list. Entries can be added to or removed from the list
// in O(1) time and with no additional memory allocations
//
// The zero value for List is an empty list ready to use
//
// To iterate over a list (where l is a List):
// 		for e := l.Front(); e != nil; e = e.Next() {
//		// do something with e
//		}
type List struct {
	head Linker
	tail Linker
}

// Reset resets list l to the empty state
func (l *List) Reset() {
	l.head = nil
	l.tail = nil
}

// Empty returns true if the list is empty
func (l *List) Empty() bool {
	return l.head == nil
}

// Front returns the first element of list l or nil
func (l *List) Front() Linker {
	return l.head
}

// Back returns the last element of list l or nil
func (l *List) Back() Linker {
	return l.tail
}

// PushBack inserts the element e at the back of list l
func (l *List) PushBack(e Linker) {
	e.SetNext(nil)
	e.SetPrev(l.tail)

	if l.tail != nil {
		l.tail.SetNext(e)
	} else {
		l.head = e
	}

	l.tail = e
}

// Remove removes e from l
func (l *List) Remove(e Linker) {
	prev := e.Prev()
	next := e.Next()

	if prev != nil {
		prev.SetNext(next)
	} else {
		l.head = next
	}

	if next != nil {
		next.SetPrev(prev)
	} else {
		l.tail = prev
	}
}

// Entry is a default implementations of Linker. Users can add anonymous fields
// of this type to their structs to make them automatically implement the
// methods needed by List
type Entry struct {
	next Linker
	prev Linker
}

// Next returns the entry that follows e in the list
func (e *Entry) Next() Linker {
	return e.next
}

// Prev returns the entry that precedes e in the list
func (e *Entry) Prev() Linker {
	return e.prev
}

// SetNext assigns 'entry' as the entry that follows e in the list
func (e *Entry) SetNext(entry Linker) {
	e.next = entry
}

// SetPrev assigns 'entry' as the entry that precedes e in the list
func (e *Entry) SetPrev(entry Linker) {
	e.prev = entry
}
