// Example program that erroneously captures the outer variable when it likely
// intends to use the parameter.

package main

type MyImpl struct{}

func (m MyImpl) Do() {}

func (m MyImpl) doThing(callback func(tx MyImpl)) {
	myImpl := MyImpl{}
	callback(myImpl)
}

type HasMyImpl struct {
	A MyImpl
}

func (h HasMyImpl) GetMyImpl() *MyImpl {
	return &h.A
}

func main() {
	outer := MyImpl{}
	outer2 := HasMyImpl{A: MyImpl{}}
	outer3 := struct{ B HasMyImpl }{B: HasMyImpl{A: MyImpl{}}}
	outerArr := [2]MyImpl{{}, {}}

	outer.doThing(func(inner MyImpl) {
		outer.Do()              // want "method call on receiver type outer not through parameter"
		outer3.B.A.Do()         // want "method call on receiver type outer3.B.A not through parameter"
		outerArr[0].Do()        // We don't flag this yet because it is a lot of extra work
		outer2.GetMyImpl().Do() // We don't flag this yet because it becomes much harder to analyze where the receiver is coming from
		inner.Do()
	})

	outer2.A.doThing(func(inner MyImpl) {
		outer2.A.Do() // want "method call on receiver type outer2.A not through parameter"
	})
}
