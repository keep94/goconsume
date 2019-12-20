package goconsume_test

import (
  "testing"

  "github.com/keep94/goconsume"
  "github.com/stretchr/testify/assert"
)

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
  consumer := goconsume.Composite(
      nilConsumer{},
      goconsume.ModFilter(
          goconsume.Slice(goconsume.AppendTo(&timesTen), 0, 100),
          func(ptr interface{}) bool {
            p := ptr.(*int)
            *p *= 10
            return true
          },
          new(int)),
      goconsume.Slice(goconsume.AppendTo(&zeroToFive), 0, 5),
      goconsume.Slice(goconsume.AppendTo(&threeToSeven), 3, 7),
      goconsume.Slice(goconsume.AppendPtrsTo(&oneToThreePtr), 1, 3),
      goconsume.Filter(
          goconsume.Slice(goconsume.AppendTo(&sevensTo28), 1, 4),
          func(ptr interface{}) bool {
            i := *ptr.(*int)
            return i % 7 == 0
          }))
  feedInts(t, consumer)
  assert.Equal([]int{0, 1, 2, 3, 4}, zeroToFive)
  assert.Equal([]int{3, 4, 5, 6}, threeToSeven)
  assert.Equal([]*int{onePtr, twoPtr}, oneToThreePtr)
  assert.Equal([]int{7, 14, 21}, sevensTo28)
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
    assert.Equal(idx, nidx)
    idx++
  }
  assert.Panics(func() {
    consumer.Consume(&idx)
  })
}

type nilConsumer struct {
}

func (n nilConsumer) CanConsume() bool {
  return false
}

func (n nilConsumer) Consume(ptr interface{}) {
  panic("Can't consume")
}
