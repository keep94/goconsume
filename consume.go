// Package goconsume provides useful ways to consume values.
package goconsume

import (
  "reflect"
)

const (
  kCantConsume = "Can't consume"
)

// Consumer consumes values. The values that Consumer consumes must
// support assignment.
type Consumer interface {

  // CanConsume returns true if this instance can consume a value
  CanConsume() bool

  // Consume consumes the value that ptr points to. Consume panics if
  // CanConsume() returns false.
  Consume(ptr interface{})
}

// ConsumeFinalizer adds a Finalize method to Consumer.
type ConsumeFinalizer interface {
  Consumer

  // Caller calls Finalize after it is done passing values to this consumer.
  // Once caller calls Finalize(), CanConsume() returns false and Consume()
  // panics.
  Finalize()
}

// AppendTo returns a Consumer that appends to aValueSlicePointer.
// aValueSlicePointer is a pointer to a slice of values supporting
// assignment.
func AppendTo(aValueSlicePointer interface{}) Consumer {
  aSliceValue := sliceValueFromP(aValueSlicePointer, false)
  return &appendConsumer{buffer: aSliceValue}
}

// AppendPtrsTo returns a Consumer that appends to aPointerSlicePointer.
// aPointerSlicePointer is a pointer to a slice of pointers to values.
func AppendPtrsTo(aPointerSlicePointer interface{}) Consumer {
  aSliceValue := sliceValueFromP(aPointerSlicePointer, true)
  aSliceType := aSliceValue.Type()
  allocType := aSliceType.Elem().Elem()
  return &appendConsumer{buffer: aSliceValue, allocType: allocType}
}

// Composite returns consumers as a single Consumer. When returned consumer
// consumes a value, each consumer in consumers that is able to consume a
// value consumes that value. CanConsume() of returned consumer returns false
// when the CanConsume() method of each consumer in consumers returns false.
func Composite(consumers ...Consumer) Consumer {
  copyOfConsumers := make([]Consumer, len(consumers))
  copy(copyOfConsumers, consumers)
  result := &multiConsumer{consumers: copyOfConsumers}
  result.filterFinished()
  return result
}

// Slice returns a Consumer that passes the start th value consumed
// inclusive to the end th value consumed exclusive onto consumer where start
// and end are zero based. The returned consumer ignores the first
// start values it consumes. After that it passes the values it consumes
// onto consumer until it has consumed end values. The CanConsume() method
// of returned consumer returns false if the CanConsume() method of the
// underlying consumer returns false or if the returned consumer has consumed
// end values. Note that if end <= start, the underlying consumer will never
// get any values.
func Slice(consumer Consumer, start, end int) Consumer {
  return &sliceConsumer{consumer: consumer, start: start, end: end}
}

// FilterFunc filters values. It returns true if the value should be
// included and false if value should be excluded. ptr points to the value
// to be filtered.
type FilterFunc func(ptr interface{}) bool

// All returns a FilterFunc that returns true for a given value if and only
// if all the filters passed to All return true for that same value.
func All(filters ...FilterFunc) FilterFunc {
  return func(ptr interface{}) bool {
    for _, filter := range filters {
      if !filter(ptr) {
        return false
      }
    }
    return true
  }
}

// ModFilter works like Filter except that the consume method of the returned
// consumer makes a defensive copy of the value passed to it before invoking
// the filter. This way the filter function can modify the value in place
// before passing it to the wrapped consumer without unwanted side effects
// for the caller. valuePtr is a pointer to the type of value being consumed.
func ModFilter(
    consumer Consumer, filter FilterFunc, valuePtr interface{}) Consumer {
  spareValuePtr :=reflect.New(reflect.TypeOf(valuePtr).Elem())
  return &filterConsumer{
      consumer: consumer,
      filter:filter,
      valuePtr: spareValuePtr.Interface(),
      value: spareValuePtr.Elem()}
}

// Filter returns a Consumer that passes only filtered values onto consumer.
// If filter returns false for a value, the returned Consumer ignores that
// value. The CanConsume() method of returned consumer returns true if and
// only if the CanConsume() method of consumer returns true.
func Filter(consumer Consumer, filter FilterFunc) Consumer {
  return &filterConsumer{
      consumer: consumer, filter: filter}
}

// Page returns a consumer that does pagination. The items in page fetched
// get stored in the slice pointed to by aValueSlicePointer.
// If there are more pages after page fetched, Page sets morePages to true;
// otherwise, it sets morePages to false. Note that the values stored at
// aValueSlicePointer and morePages are undefined until caller calls
// Finalize() on returned ConsumeFinalizer.
func Page(
    zeroBasedPageNo int,
    itemsPerPage int,
    aValueSlicePointer interface{},
    morePages *bool) ConsumeFinalizer {
  ensureCapacity(aValueSlicePointer, itemsPerPage + 1)
  truncateTo(aValueSlicePointer, 0)
  consumer := AppendTo(aValueSlicePointer)
  consumer = Slice(
      consumer,
      zeroBasedPageNo * itemsPerPage,
      (zeroBasedPageNo + 1) * itemsPerPage + 1)
  return &pageConsumer{
      Consumer: consumer,
      itemsPerPage: itemsPerPage,
      aValueSlicePointer: aValueSlicePointer,
      morePages: morePages}
}

type pageConsumer struct {
  Consumer
  itemsPerPage int
  aValueSlicePointer interface{}
  morePages *bool
}

func (p *pageConsumer) Finalize() {
  p.Consumer = nilConsumer{}
  if lengthOfSlicePtr(p.aValueSlicePointer) == p.itemsPerPage + 1 {
    *p.morePages = true
    truncateTo(p.aValueSlicePointer, p.itemsPerPage)
  } else {
    *p.morePages = false
  }
}

func ensureCapacity(aSlicePointer interface{}, capacity int) {
  value := reflect.ValueOf(aSlicePointer).Elem()
  if value.Cap() < capacity {
    typ := value.Type()
    value.Set(reflect.MakeSlice(typ, 0, capacity))
  }
}

func truncateTo(aSlicePointer interface{}, newLength int) {
  value := reflect.ValueOf(aSlicePointer).Elem()
  value.Set(value.Slice(0, newLength))
}

func lengthOfSlicePtr(aSlicePointer interface{}) int {
  value := reflect.ValueOf(aSlicePointer).Elem()
  return value.Len()
}

type filterConsumer struct {
  consumer Consumer
  filter FilterFunc
  valuePtr interface{}
  value reflect.Value
}

func (f *filterConsumer) CanConsume() bool {
  return f.consumer.CanConsume()
}

func (f *filterConsumer) Consume(ptr interface{}) {
  if !f.CanConsume() {
    panic(kCantConsume)
  }
  if f.valuePtr != nil {
    f.value.Set(reflect.ValueOf(ptr).Elem())
    ptr = f.valuePtr
  }
  if f.filter(ptr) {
    f.consumer.Consume(ptr)
  }
}

type sliceConsumer struct {
  consumer Consumer
  start int
  end int
  idx int
}

func (s *sliceConsumer) CanConsume() bool {
  return s.consumer.CanConsume() && s.idx < s.end
}

func (s *sliceConsumer) Consume(ptr interface{}) {
  if !s.CanConsume() {
    panic(kCantConsume)
  }
  if s.idx >= s.start {
    s.consumer.Consume(ptr)
  }
  s.idx++
}

type multiConsumer struct {
  consumers []Consumer
}

func (m *multiConsumer) CanConsume() bool {
  return len(m.consumers) > 0
}

func (m *multiConsumer) Consume(ptr interface{}) {
  if !m.CanConsume() {
    panic(kCantConsume)
  }
  for _, consumer := range m.consumers {
    consumer.Consume(ptr)
  }
  m.filterFinished()
}

func (m *multiConsumer) filterFinished() {
  idx := 0
  for i := range m.consumers {
    if m.consumers[i].CanConsume() {
      m.consumers[idx] = m.consumers[i]
      idx++
    }
  }
  m.consumers = m.consumers[0:idx]
}

type appendConsumer struct {
  buffer reflect.Value
  allocType reflect.Type
}

func (a *appendConsumer) CanConsume() bool {
  return true
}

func (a *appendConsumer) Consume(ptr interface{}) {
  valueToConsume := reflect.ValueOf(ptr).Elem()
  if a.allocType == nil {
    a.buffer.Set(reflect.Append(a.buffer, valueToConsume))
  } else {
    newPtr := reflect.New(a.allocType)
    newPtr.Elem().Set(valueToConsume)
    a.buffer.Set(reflect.Append(a.buffer, newPtr))
  }
}

func sliceValueFromP(
    aSlicePointer interface{}, sliceOfPtrs bool) reflect.Value {
  resultPtr := reflect.ValueOf(aSlicePointer)
  if resultPtr.Type().Kind() != reflect.Ptr {
    panic("A pointer to a slice is expected.")
  }
  return checkSliceValue(resultPtr.Elem(), sliceOfPtrs)
}

func checkSliceValue(
    result reflect.Value, sliceOfPtrs bool) reflect.Value {
  if result.Kind() != reflect.Slice {
    panic("a slice is expected.")
  }
  if sliceOfPtrs && result.Type().Elem().Kind() != reflect.Ptr {
    panic("a slice of pointers is expected.")
  }
  return result
}

type nilConsumer struct {
}

func (n nilConsumer) CanConsume() bool {
  return false
}

func (n nilConsumer) Consume(ptr interface{}) {
  panic(kCantConsume)
}
