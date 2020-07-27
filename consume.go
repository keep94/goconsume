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

// ConsumerFunc can always consume.
type ConsumerFunc func(ptr interface{})

// Consume invokes c, this function.
func (c ConsumerFunc) Consume(ptr interface{}) {
  c(ptr)
}

// CanConsume always returns true.
func (c ConsumerFunc) CanConsume() bool {
  return true
}

// MustCanConsume panics if c cannot consume
func MustCanConsume(c Consumer) {
  if !c.CanConsume() {
    panic(kCantConsume)
  }
}

// Nil returns a consumer that consumes nothing. Calling CanConsume() on
// returned consumer returns false, and calling Consume() on returned
// consumer panics.
func Nil() Consumer {
  return nilConsumer{}
}

// AppendTo returns a Consumer that appends consumed values to the slice
// pointed to by aValueSlicePointer. aValueSlicePointer is a pointer to a
// slice of values supporting assignment. The CanConsume method of returned
// consumer always returns true.
func AppendTo(aValueSlicePointer interface{}) Consumer {
  aSliceValue := sliceValueFromP(aValueSlicePointer, false)
  return &appendConsumer{buffer: aSliceValue}
}

// AppendPtrsTo returns a Consumer that appends consumed values to the slice
// pointed to by aPointerSlicePointer. Each time the returned Consumer
// consumes a value, it allocates a new value on the heap, copies the
// consumed value to that allocated value, and finally appends the pointer
// to the newly allocated value to the slice pointed to by
// aPointerSlicePointer. aPointerSlicePointer is a pointer to a slice of
// pointers to values supporting assignment. The CanConsume method of
// returned consumer always returns true.
func AppendPtrsTo(aPointerSlicePointer interface{}) Consumer {
  aSliceValue := sliceValueFromP(aPointerSlicePointer, true)
  aSliceType := aSliceValue.Type()
  allocType := aSliceType.Elem().Elem()
  return &appendConsumer{buffer: aSliceValue, allocType: allocType}
}

// Compose returns the consumers passed to it as a single Consumer. When
// returned consumer consumes a value, each consumer passed in that is able to
// consume a value consumes that value. CanConsume() of returned consumer
// returns false when the CanConsume() method of each consumer passed in
// returns false. Caller should use the returned Consumer instead of the
// passed in consumers. Otherwise the CanConsume method of returned consumer
// may not work correctly. The consumers passed to Compose must not modify
// values being consumed. Wrap any such consumer that modifies the values it
// consumes with the Copy method or use ComposeWithCopy.
func Compose(consumers ...Consumer) Consumer {
  consumerList := make([]Consumer, len(consumers))
  copy(consumerList, consumers)
  result := &multiConsumer{consumers: consumerList}
  result.filterFinished()
  return result
}

// Copy returns a consumer that copies the value it consumes using assignment
// and passes that copy onto consumer. valuePtr is a pointer to the type of
// value being consumed. Callers generally pass nil for it like this:
// (*TypeBeingConsumed)(nil).
func Copy(consumer Consumer, valuePtr interface{}) Consumer {
  spareValuePtr := reflect.New(reflect.TypeOf(valuePtr).Elem())
  return &consumerWithValue{
      Consumer: consumer,
      valuePtr: spareValuePtr.Interface(),
      value: spareValuePtr.Elem()}
}

// ComposeWithCopy is like Compose except that ComposeWithCopy passes a
// separate copy using assignment of the value being consumed to each of
// the consumers. valuePtr is a pointer to the type of value being consumed.
// Callers generally pass nil for it like this:
// (*TypeBeingConsumed)(nil).
func ComposeWithCopy(consumers []Consumer, valuePtr interface{}) Consumer {
  consumerList := make([]Consumer, len(consumers))
  for i := range consumers {
    consumerList[i] = Copy(consumers[i], valuePtr)
  }
  result := &multiConsumer{consumers: consumerList}
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
  filterList := make([]FilterFunc, len(filters))
  copy(filterList, filters)
  return func(ptr interface{}) bool {
    for _, filter := range filterList {
      if !filter(ptr) {
        return false
      }
    }
    return true
  }
}

// Filter returns a Consumer that passes only filtered values onto consumer.
// If filter returns false for a value, the returned Consumer ignores that
// value.  The CanConsume() method of returned Consumer returns true if and
// only if the CanConsume() method of consumer returns true.
func Filter(consumer Consumer, filter FilterFunc) Consumer {
  return &filterConsumer{
      Consumer: consumer,
      filter:filter}
}

// MapFunc maps values. It applies a transformation to srcPtr and stores the
// result in destPtr while leaving srcPtr unchanged. It returns true on
// success or false if no transformation took place.
type MapFunc func(srcPtr, destPtr interface{}) bool

// Map returns a Consumer that passes only mapped values onto the consumer
// parameter. The pointer passsed to returned Consumer gets passed as srcPtr
// to mapfunc. The destPtr parameter of mapfunc gets passed to the
// Consume method of the consumer parameter. If mapfunc returns false,
// the returned Consumer ignores the parameter passed to it and will not call
// the Consume method of the consumer parameter. valuePtr is pointer to the
// type of value that the consumer parameter consumes. It is used to create
// the value that destPtr passed to mapfunc points to. Callers
// generally pass nil for it like this: (*TypeBeingConsumed)(nil).
// CanConsume() of returned consumer returns true if and only if the
// CanConsume() method of the consumer parameter returns true.
func Map(consumer Consumer, mapfunc MapFunc, valuePtr interface{}) Consumer {
  spareValuePtr := reflect.New(reflect.TypeOf(valuePtr).Elem())
  return &mapConsumer{
      Consumer: consumer,
      mapfunc: mapfunc,
      valuePtr: spareValuePtr.Interface()}
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
  finalized bool
}

func (p *pageConsumer) Finalize() {
  if p.finalized {
    return
  }
  p.finalized = true
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
  Consumer
  filter FilterFunc
}

func (f *filterConsumer) Consume(ptr interface{}) {
  MustCanConsume(f)
  if f.filter(ptr) {
    f.Consumer.Consume(ptr)
  }
}

type mapConsumer struct {
  Consumer
  mapfunc MapFunc
  valuePtr interface{}
}

func (m *mapConsumer) Consume(ptr interface{}) {
  MustCanConsume(m)
  if m.mapfunc(ptr, m.valuePtr) {
    m.Consumer.Consume(m.valuePtr)
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
  MustCanConsume(s)
  if s.idx >= s.start {
    s.consumer.Consume(ptr)
  }
  s.idx++
}

type consumerWithValue struct {
  Consumer
  valuePtr interface{}
  value reflect.Value
}

func (c *consumerWithValue) Consume(ptr interface{}) {
  MustCanConsume(c)
  c.value.Set(reflect.ValueOf(ptr).Elem())
  c.Consumer.Consume(c.valuePtr)
}

type multiConsumer struct {
  consumers []Consumer
}

func (m *multiConsumer) CanConsume() bool {
  return len(m.consumers) > 0
}

func (m *multiConsumer) Consume(ptr interface{}) {
  MustCanConsume(m)
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
  for i := idx; i < len(m.consumers); i++ {
    m.consumers[i] = nil
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
