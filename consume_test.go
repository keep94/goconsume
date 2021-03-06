package goconsume_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/keep94/goconsume"
	"github.com/stretchr/testify/assert"
)

func TestNil(t *testing.T) {
	assert := assert.New(t)
	consumer := goconsume.Nil()
	assert.False(consumer.CanConsume())
	assert.Panics(func() { consumer.Consume(new(int)) })
}

func TestPageConsumer(t *testing.T) {
	assert := assert.New(t)
	var arr []int
	var morePages bool
	pager := goconsume.Page(0, 5, &arr, &morePages)
	feedInts(t, pager)
	pager.Finalize()
	pager.Finalize() // check idempotency of Finalize
	assert.Equal([]int{0, 1, 2, 3, 4}, arr)
	assert.True(morePages)
	assert.False(pager.CanConsume())
	assert.Panics(func() { pager.Consume(new(int)) })

	pager = goconsume.Page(3, 5, &arr, &morePages)
	feedInts(t, pager)
	pager.Finalize()
	assert.Equal([]int{15, 16, 17, 18, 19}, arr)
	assert.True(morePages)
	assert.False(pager.CanConsume())
	assert.Panics(func() { pager.Consume(new(int)) })

	pager = goconsume.Page(2, 5, &arr, &morePages)
	feedInts(t, goconsume.Slice(pager, 0, 15))
	pager.Finalize()
	assert.Equal([]int{10, 11, 12, 13, 14}, arr)
	assert.False(morePages)
	assert.False(pager.CanConsume())
	assert.Panics(func() { pager.Consume(new(int)) })

	pager = goconsume.Page(2, 5, &arr, &morePages)
	feedInts(t, goconsume.Slice(pager, 0, 11))
	pager.Finalize()
	assert.Equal([]int{10}, arr)
	assert.False(morePages)
	assert.False(pager.CanConsume())
	assert.Panics(func() { pager.Consume(new(int)) })

	pager = goconsume.Page(2, 5, &arr, &morePages)
	feedInts(t, goconsume.Slice(pager, 0, 10))
	pager.Finalize()
	assert.Equal([]int{}, arr)
	assert.False(morePages)
	assert.False(pager.CanConsume())
	assert.Panics(func() { pager.Consume(new(int)) })
}

func TestComposeUseIndividual(t *testing.T) {
	assert := assert.New(t)
	var strs []string
	var ints []int
	consumerOne := goconsume.MapFilter(
		goconsume.Slice(goconsume.AppendTo(&strs), 0, 1),
		func(src *int, dest *string) bool {
			*dest = strconv.Itoa(*src)
			return true
		})
	consumerThree := goconsume.Slice(goconsume.AppendTo(&ints), 0, 3)
	composite := goconsume.Compose(consumerOne, consumerThree, goconsume.Nil())
	assert.True(composite.CanConsume())
	i := 1
	composite.Consume(&i)
	assert.True(composite.CanConsume())
	i = 2
	composite.Consume(&i)
	assert.True(composite.CanConsume())
	i = 3

	// Use up individual consumer
	consumerThree.Consume(&i)

	// Now the composite consumer should return false
	assert.False(composite.CanConsume())

	assert.Equal([]string{"1"}, strs)
	assert.Equal([]int{1, 2, 3}, ints)
}

func TestCompose(t *testing.T) {
	assert := assert.New(t)
	var zeroToFive []int
	var timesTen []int
	consumer := goconsume.Compose(
		goconsume.Nil(),
		goconsume.Copy(
			goconsume.Filter(
				goconsume.Slice(goconsume.AppendTo(&timesTen), 0, 100),
				func(ptr interface{}) bool {
					p := ptr.(*int)
					*p *= 10
					return true
				}), (*int)(nil)),
		goconsume.Slice(goconsume.AppendTo(&zeroToFive), 0, 5),
	)
	feedInts(t, consumer)
	assert.Equal([]int{0, 1, 2, 3, 4}, zeroToFive)
}

func TestConsumer(t *testing.T) {
	assert := assert.New(t)
	var zeroToFive []int
	var threeToSeven []int
	var sevensTo28 []int
	var timesTen []int
	var oneToThreePtr []*int
	onePtr := new(int)
	twoPtr := new(int)
	*onePtr = 1
	*twoPtr = 2
	consumer := goconsume.ComposeWithCopy(
		[]goconsume.Consumer{
			goconsume.Nil(),
			goconsume.Filter(
				goconsume.Slice(goconsume.AppendTo(&timesTen), 0, 100),
				func(ptr interface{}) bool {
					p := ptr.(*int)
					*p *= 10
					return true
				}),
			goconsume.Slice(goconsume.AppendTo(&zeroToFive), 0, 5),
			goconsume.Slice(goconsume.AppendTo(&threeToSeven), 3, 7),
			goconsume.Slice(goconsume.AppendPtrsTo(&oneToThreePtr), 1, 3),
			goconsume.Filter(
				goconsume.Slice(goconsume.AppendTo(&sevensTo28), 1, 4),
				func(ptr interface{}) bool {
					i := *ptr.(*int)
					return i%7 == 0
				})},
		(*int)(nil))
	feedInts(t, consumer)
	assert.Equal([]int{0, 1, 2, 3, 4}, zeroToFive)
	assert.Equal([]int{3, 4, 5, 6}, threeToSeven)
	assert.Equal([]*int{onePtr, twoPtr}, oneToThreePtr)
	assert.Equal([]int{7, 14, 21}, sevensTo28)
	assert.Equal(10, timesTen[1])
	assert.Equal(20, timesTen[2])
}

func TestSlice(t *testing.T) {
	assert := assert.New(t)
	var zeroToFive []int
	feedInts(t, goconsume.Slice(goconsume.AppendTo(&zeroToFive), 0, 5))
	assert.Equal([]int{0, 1, 2, 3, 4}, zeroToFive)
}

func TestFilter(t *testing.T) {
	assert := assert.New(t)
	var sevensTo28 []int
	feedInts(t, goconsume.Filter(
		goconsume.Slice(goconsume.AppendTo(&sevensTo28), 1, 4),
		func(ptr interface{}) bool {
			i := *ptr.(*int)
			return i%7 == 0
		}))
	assert.Equal([]int{7, 14, 21}, sevensTo28)
}

func TestMap(t *testing.T) {
	assert := assert.New(t)
	var zeroTo10By2 []string
	feedInts(t, goconsume.Map(
		goconsume.Slice(goconsume.AppendTo(&zeroTo10By2), 0, 6),
		func(srcPtr, destPtr interface{}) bool {
			src := *srcPtr.(*int)
			if src%2 == 1 {
				return false
			}
			*destPtr.(*string) = strconv.Itoa(src)
			return true
		},
		(*string)(nil)))
	assert.Equal([]string{"0", "2", "4", "6", "8", "10"}, zeroTo10By2)
}

func TestMapFilter(t *testing.T) {
	assert := assert.New(t)
	var zeroTo150By30 []string
	feedInts(t, goconsume.MapFilter(
		goconsume.Slice(goconsume.AppendTo(&zeroTo150By30), 0, 6),
		goconsume.NewApplier(),
		goconsume.NewApplier(
			func(ptr *int) bool {
				return (*ptr)%2 == 0
			},
			goconsume.NewApplier(func(ptr *int) bool {
				return (*ptr)%3 == 0
			}),
		),
		func(srcPtr *int, destPtr *string) bool {
			if (*srcPtr)%5 != 0 {
				return false
			}
			*destPtr = strconv.Itoa(*srcPtr)
			return true
		}))
	assert.Equal([]string{"0", "30", "60", "90", "120", "150"}, zeroTo150By30)
}

func ExampleMapFilter() {
	var evens []string
	consumer := goconsume.MapFilter(
		goconsume.AppendTo(&evens),
		func(ptr *int) bool {
			return (*ptr)%2 == 0
		},
		func(src *int, dest *string) bool {
			*dest = strconv.Itoa(*src)
			return true
		},
	)
	ints := []int{1, 2, 4}
	for _, i := range ints {
		if consumer.CanConsume() {
			consumer.Consume(&i)
		}
	}
	fmt.Println(evens)
	// Output: [2 4]
}

func TestAllImmutability(t *testing.T) {
	assert := assert.New(t)
	multipleOfTwo := func(ptr interface{}) bool {
		return (*ptr.(*int))%2 == 0
	}
	multipleOfThree := func(ptr interface{}) bool {
		return (*ptr.(*int))%3 == 0
	}
	filters := []goconsume.FilterFunc{multipleOfTwo, multipleOfThree}
	allFilter := goconsume.All(filters...)
	x := 6
	assert.True(allFilter(&x))
	x = 10
	assert.False(allFilter(&x))
	filters[1] = func(ptr interface{}) bool {
		return (*ptr.(*int))%5 == 0
	}

	// Even though we mutated the filters array allFilter should still behave
	// the same as before.
	x = 6
	assert.True(allFilter(&x))
	x = 10
	assert.False(allFilter(&x))
}

func TestAll(t *testing.T) {
	assert := assert.New(t)
	multipleOfTwo := func(ptr interface{}) bool {
		return (*ptr.(*int))%2 == 0
	}
	multipleOfThree := func(ptr interface{}) bool {
		return (*ptr.(*int))%3 == 0
	}
	filter := goconsume.All(multipleOfTwo, multipleOfThree)
	x := 1
	assert.False(filter(&x))
	x = 2
	assert.False(filter(&x))
	x = 3
	assert.False(filter(&x))
	x = 6
	assert.True(filter(&x))
}

func feedInts(t *testing.T, consumer goconsume.Consumer) {
	assert := assert.New(t)
	idx := 0
	for consumer.CanConsume() {
		nidx := idx
		consumer.Consume(&nidx)
		idx++
	}
	assert.Panics(func() {
		consumer.Consume(&idx)
	})
}
