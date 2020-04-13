package goconsume_test

import (
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
  assert.Equal([]int{0,1,2,3,4}, arr)
  assert.True(morePages)
  assert.False(pager.CanConsume())
  assert.Panics(func() { pager.Consume(new(int)) })

  pager = goconsume.Page(3, 5, &arr, &morePages)
  feedInts(t, pager)
  pager.Finalize()
  assert.Equal([]int{15,16,17,18,19}, arr)
  assert.True(morePages)
  assert.False(pager.CanConsume())
  assert.Panics(func() { pager.Consume(new(int)) })

  pager = goconsume.Page(2, 5, &arr, &morePages)
  feedInts(t, goconsume.Slice(pager, 0, 15))
  pager.Finalize()
  assert.Equal([]int{10,11,12,13,14}, arr)
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
                return i % 7 == 0
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
        return i % 7 == 0
      }))
  assert.Equal([]int{7, 14, 21}, sevensTo28)
}

func TestAll(t *testing.T) {
  assert := assert.New(t)
  multipleOfTwo := func(ptr interface{}) bool {
    return (*ptr.(*int)) % 2 == 0
  }
  multipleOfThree := func(ptr interface{}) bool {
    return (*ptr.(*int)) % 3 == 0
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
