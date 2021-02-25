// Package goconsume provides useful ways to consume values.
package goconsume

import (
	"reflect"

	"github.com/keep94/common"
)

const (
	kCantConsume         = "Can't consume"
	kParamMustReturnBool = "Parameter must return bool"
)

// Consumer consumes values. The values that Consumer consumes must
// support assignment.
type Consumer interface {

	// CanConsume returns true if this instance can consume a value
	// Once CanConsume returns false, it should always return false.
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
// returns false.
func Compose(consumers ...Consumer) Consumer {
	clen := len(consumers)
	switch clen {
	case 0:
		return nilConsumer{}
	case 1:
		return consumers[0]
	default:
		consumerList := make([]Consumer, clen)
		copy(consumerList, consumers)
		return &multiConsumer{consumers: consumerList}
	}
}

// Copy returns a consumer that copies the value it consumes using assignment
// and passes that copy onto consumer. valuePtr is a pointer to the type of
// value being consumed. Callers generally pass nil for it like this:
// (*TypeBeingConsumed)(nil).
//
// Deprecated: Use MapFilter.
func Copy(consumer Consumer, valuePtr interface{}) Consumer {
	spareValuePtr := reflect.New(reflect.TypeOf(valuePtr).Elem())
	return &consumerWithValue{
		Consumer: consumer,
		valuePtr: spareValuePtr.Interface(),
		value:    spareValuePtr.Elem()}
}

// ComposeWithCopy is like Compose except that ComposeWithCopy passes a
// separate copy using assignment of the value being consumed to each of
// the consumers. valuePtr is a pointer to the type of value being consumed.
// Callers generally pass nil for it like this:
// (*TypeBeingConsumed)(nil).
//
// Deprecated: Use MapFilter.
func ComposeWithCopy(consumers []Consumer, valuePtr interface{}) Consumer {
	consumerList := make([]Consumer, len(consumers))
	for i := range consumers {
		consumerList[i] = Copy(consumers[i], valuePtr)
	}
	return Compose(consumerList...)
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
//
// Deprecated: Use Applier.
type FilterFunc func(ptr interface{}) bool

// All returns a FilterFunc that returns true for a given value if and only
// if all the filters passed to All return true for that same value.
//
// Deprecated: Use NewApplier.
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
//
// Deprecated: Use MapFilter.
func Filter(consumer Consumer, filter FilterFunc) Consumer {
	return &filterConsumer{
		Consumer: consumer,
		filter:   filter}
}

// MapFunc maps values. It applies a transformation to srcPtr and stores the
// result in destPtr while leaving srcPtr unchanged. It returns true on
// success or false if no transformation took place.
//
// Deprecated: Use MapFilter.
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
//
// Deprecated: Use MapFilter.
func Map(consumer Consumer, mapfunc MapFunc, valuePtr interface{}) Consumer {
	spareValuePtr := reflect.New(reflect.TypeOf(valuePtr).Elem())
	return &mapConsumer{
		Consumer: consumer,
		mapfunc:  mapfunc,
		valuePtr: spareValuePtr.Interface()}
}

// Interface Applier represents zero or more functions like the ones passed
// to MapFilter chained together.
type Applier interface {

	// Apply applies the chained filter and map functions to what ptr points
	// to while leaving it unchanged. Apply returns nil if ptr should be
	// filtered out; returns ptr itself; or returns a pointer to a new value.
	// If Apply returns a pointer to a new value, the new value gets
	// overwritten with each call to Apply.
	Apply(ptr interface{}) interface{}

	private()
}

// NewApplier creates an Applier from multiple functions like the ones passed
// to MapFilter chained together. The returned Applier can be passed as a
// parameter to MapFilter or to NewApplier.
func NewApplier(funcs ...interface{}) Applier {
	result := make(sliceApplier, len(funcs))
	for i := range result {
		result[i] = newApplier(funcs[i])
	}
	return common.Join(result, sliceApplier(nil), nilApplier{}).(Applier)
}

// MapFilter returns a Consumer that passes only filtered and mapped
// values onto the consumer parameter. The returned Consumer applies
// each function in funcs to the value passed to its Consume method.
// The resulting value is then passed to the Consume method of the
// consumer parameter. Each function in func returns a bool and takes
// one or two pointer arguments to values. If a function returns false,
// it means that the value passed to it should not be consumed. The one
// argument functions never change the value passed to it as they are
// simple filters. The 2 argument functions are mappers. They leave their
// first argument unchanged, but use it to set their second argument.
// This second argument is what gets passed to the next function in funcs
// or to consumer if it is the last function in funcs.
//
// The NewApplier function can return an Applier which represents zero or
// more of these functions chained together. Applier instances can be passed
// as parameters to MapFilter just like the functions mentioned above.
func MapFilter(consumer Consumer, funcs ...interface{}) Consumer {
	mapFilters := NewApplier(funcs...)
	if _, ok := mapFilters.(nilApplier); ok {
		return consumer
	}
	if mf, ok := consumer.(*mapFilterConsumer); ok {
		return &mapFilterConsumer{
			Consumer:   mf.Consumer,
			mapFilters: NewApplier(mapFilters, mf.mapFilters),
		}
	}
	return &mapFilterConsumer{
		Consumer:   consumer,
		mapFilters: mapFilters,
	}
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
	ensureCapacity(aValueSlicePointer, itemsPerPage+1)
	truncateTo(aValueSlicePointer, 0)
	consumer := AppendTo(aValueSlicePointer)
	consumer = Slice(
		consumer,
		zeroBasedPageNo*itemsPerPage,
		(zeroBasedPageNo+1)*itemsPerPage+1)
	return &pageConsumer{
		Consumer:           consumer,
		itemsPerPage:       itemsPerPage,
		aValueSlicePointer: aValueSlicePointer,
		morePages:          morePages}
}

type pageConsumer struct {
	Consumer
	itemsPerPage       int
	aValueSlicePointer interface{}
	morePages          *bool
	finalized          bool
}

func (p *pageConsumer) Finalize() {
	if p.finalized {
		return
	}
	p.finalized = true
	p.Consumer = nilConsumer{}
	if lengthOfSlicePtr(p.aValueSlicePointer) == p.itemsPerPage+1 {
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
	mapfunc  MapFunc
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
	start    int
	end      int
	idx      int
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
	value    reflect.Value
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
	m.filterFinished()
	return len(m.consumers) > 0
}

func (m *multiConsumer) Consume(ptr interface{}) {
	MustCanConsume(m)
	for _, consumer := range m.consumers {
		consumer.Consume(ptr)
	}
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
	buffer    reflect.Value
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

func newApplier(f interface{}) Applier {
	if a, ok := f.(Applier); ok {
		return a
	}
	fvalue := reflect.ValueOf(f)
	ftype := reflect.TypeOf(f)
	validateFuncType(ftype)
	numIn := ftype.NumIn()
	if numIn == 1 {
		return &filterApplier{
			value: fvalue,
		}
	} else if numIn == 2 {
		spareValue := reflect.New(ftype.In(1).Elem())
		return &mapApplier{
			value:   fvalue,
			result:  spareValue,
			iresult: spareValue.Interface(),
		}
	} else {
		panic("Function parameter must take 1 or 2 parameters")
	}
}

func validateFuncType(ftype reflect.Type) {
	if ftype.Kind() != reflect.Func {
		panic("Parameter must be a function")
	}
	if ftype.NumOut() != 1 {
		panic(kParamMustReturnBool)
	}
	if ftype.Out(0) != reflect.TypeOf(true) {
		panic(kParamMustReturnBool)
	}
	numIn := ftype.NumIn()
	for i := 0; i < numIn; i++ {
		if ftype.In(i).Kind() != reflect.Ptr {
			panic("Function parameter must accept pointer arguments")
		}
	}
}

type filterApplier struct {
	value reflect.Value
}

func (f *filterApplier) Apply(ptr interface{}) interface{} {
	params := [...]reflect.Value{reflect.ValueOf(ptr)}
	if f.value.Call(params[:])[0].Bool() {
		return ptr
	}
	return nil
}

func (f *filterApplier) private() {
}

type mapApplier struct {
	value   reflect.Value
	result  reflect.Value
	iresult interface{}
}

func (m *mapApplier) Apply(ptr interface{}) interface{} {
	params := [...]reflect.Value{reflect.ValueOf(ptr), m.result}
	if m.value.Call(params[:])[0].Bool() {
		return m.iresult
	}
	return nil
}

func (m *mapApplier) private() {
}

type sliceApplier []Applier

func (s sliceApplier) Apply(ptr interface{}) interface{} {
	for _, a := range s {
		ptr = a.Apply(ptr)
		if ptr == nil {
			return nil
		}
	}
	return ptr
}

func (s sliceApplier) private() {
}

type nilApplier struct {
}

func (n nilApplier) Apply(ptr interface{}) interface{} {
	return ptr
}

func (n nilApplier) private() {
}

type mapFilterConsumer struct {
	Consumer
	mapFilters Applier
}

func (m *mapFilterConsumer) Consume(ptr interface{}) {
	MustCanConsume(m)
	ptr = m.mapFilters.Apply(ptr)
	if ptr == nil {
		return
	}
	m.Consumer.Consume(ptr)
}
