package main

import "external_pkg/mypkg"

func doThing(callback func(tx mypkg.MyInterface)) {
	myImpl := mypkg.MyImpl{}
	callback(&myImpl)
}

type HasMyImpl struct {
	A mypkg.MyImpl
}

func (h HasMyImpl) GetMyImpl() *mypkg.MyImpl {
	return &h.A
}

func main() {
	outer := mypkg.MyImpl{}
	outer2 := HasMyImpl{A: mypkg.MyImpl{}}
	outer3 := struct{ B HasMyImpl }{B: HasMyImpl{A: mypkg.MyImpl{}}}
	outerArr := [2]mypkg.MyImpl{{}, {}}
	doThing(func(inner mypkg.MyInterface) {
		outer.Do()              // want "captured variable outer implements interface MyInterface"
		outer2.A.Do()           // want "captured variable outer2.A implements interface MyInterface"
		outer3.B.A.Do()         // want "captured variable outer3.B.A implements interface MyInterface"
		outerArr[0].Do()        // We don't flag this yet because it is a lot of extra work
		outer2.GetMyImpl().Do() // We don't flag this yet because it becomes much harder to analyze where the receiver is coming from
		inner.Do()
	})
}
